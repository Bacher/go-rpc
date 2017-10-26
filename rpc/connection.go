package rpc

import (
	"encoding/binary"
	"errors"
	"github.com/golang/protobuf/proto"
	"log"
	"net"
	"gorpc/protocol"
	"sync"
	"time"
)

type RequestHandler func(string, []byte) ([]byte, error)
type closeHandler func()

type RequestResults struct {
	err  error
	data []byte
}

var WriteTimeout = errors.New("socket write timeout")
var ResponseTimeout = errors.New("response timeout")
var ApiNotFound = errors.New("api not found")
var RemoteError = errors.New("remote error")
var Closed = errors.New("connection closed")
var NoHandler = errors.New("has not handler")

const REQUEST_TIMEOUT = 10 * time.Second

func NewConnection(requestHandler RequestHandler) *Connection {
	return &Connection{
		nil,
		make(map[uint32]chan *waitResponse),
		0,
		&sync.Mutex{},
		&sync.RWMutex{},
		false,
		false,
		false,
		requestHandler,
		nil,
		make(chan []byte),
	}
}

type waitResponse struct {
	err  error
	data []byte
}

type Connection struct {
	con            net.Conn
	wait           map[uint32]chan *waitResponse
	lastId         uint32
	idMutex        *sync.Mutex
	waitMutex      *sync.RWMutex
	connected      bool
	closing        bool
	closed         bool
	requestHandler RequestHandler
	closeHandler   closeHandler
	writeCh        chan []byte
}

func (c *Connection) Connect() error {
	con, err := net.Dial("tcp", "localhost:9999")

	if err != nil {
		return err
	}

	c.Link(con)

	return nil
}

func (c *Connection) Link(con net.Conn) {
	c.con = con
	c.connected = true

	go c.startWriteLoop()

	go c.handle()
}

func (c *Connection) handle() {
	sizeBuffer := make([]byte, 4)

	for {
		_, err := c.con.Read(sizeBuffer)

		if err != nil {
			c.errorClose(Closed)
			return
		}

		packageSize := int(binary.BigEndian.Uint32(sizeBuffer))

		messageBuffer := make([]byte, packageSize)

		var receiveSize = 0

		for {
			curReceiveSize, err := c.con.Read(messageBuffer[receiveSize:])

			if err != nil {
				c.errorClose(Closed)
				return
			}

			receiveSize += curReceiveSize

			if receiveSize == packageSize {
				break
			}
		}

		var msg pb.Message
		err = proto.Unmarshal(messageBuffer, &msg)

		if err != nil {
			c.errorClose(err)
			return
		}

		switch msg.Type {
		case pb.TYPE_PING:
			break
		case pb.TYPE_REQUEST:
			go c.response(msg.Id, msg.GetRequest())
			break
		case pb.TYPE_RESPONSE:
			go c.handleResponse(&msg, msg.GetResponse())
		}
	}
}

func (c *Connection) Close() {
	c.waitMutex.RLock()
	count := len(c.wait)
	c.waitMutex.RUnlock()

	if count == 0 {
		c.close()
	} else {
		c.closing = true
	}
}

func (c *Connection) errorClose(err error) {
	c.close()
}

func (c *Connection) close() {
	if !c.closed {
		c.closed = true
		close(c.writeCh)

		if c.closeHandler != nil {
			defer c.closeHandler()
		}

		c.waitMutex.Lock()
		for _, ch := range c.wait {
			ch <- &waitResponse{Closed, nil}
		}
		c.wait = nil
		c.waitMutex.Unlock()

		c.con.Close()
		c.con = nil
	}
}

func (c *Connection) getNextMessageId() uint32 {
	c.idMutex.Lock()
	c.lastId++
	id := c.lastId
	c.idMutex.Unlock()

	return id
}

func (c *Connection) Request(apiName string, params proto.Message) ([]byte, error) {
	if !c.connected || c.closed || c.closing {
		return nil, Closed
	}

	id := c.getNextMessageId()

	paramsBuffer, _ := proto.Marshal(params)

	msg := &pb.Message{id, pb.TYPE_REQUEST, &pb.Message_Request{&pb.Request{apiName, paramsBuffer}}}
	msgBuffer, err := proto.Marshal(msg)

	if err != nil {
		return nil, err
	}

	waitCh := make(chan *waitResponse)
	c.waitMutex.Lock()
	c.wait[id] = waitCh
	c.waitMutex.Unlock()

	c.writeBuffer(msgBuffer)

	requestTimeout := time.NewTimer(REQUEST_TIMEOUT)

	select {
	case <-requestTimeout.C:
		c.waitMutex.Lock()
		delete(c.wait, id)
		c.waitMutex.Unlock()
		return nil, ResponseTimeout
	case response := <-waitCh:
		requestTimeout.Stop()

		if response.err != nil {
			return nil, response.err
		} else {
			return response.data, nil
		}
	}
}

func (c *Connection) response(requestId uint32, request *pb.Request) {
	var err error = nil
	var resultBuffer []byte = nil

	if c.requestHandler == nil {
		log.Printf("Try to request \"%s\" on noApi handler server\n", request.Name)
		err = NoHandler

	} else if c.closing {
		err = Closed

	} else {
		startTs := time.Now()
		resultBuffer, err = c.requestHandler(request.Name, request.Params)
		elapsed := time.Since(startTs)

		if elapsed > REQUEST_TIMEOUT {
			log.Printf("[RPC] Request aborted. Too long response for \"%s\" [%ds]\n", request.Name, elapsed/time.Second)
			return
		}
	}

	var response *pb.Response

	if err != nil {
		response = &pb.Response{requestId, true, nil}
	} else {
		response = &pb.Response{requestId, false, resultBuffer}
	}

	msg := &pb.Message{c.getNextMessageId(), pb.TYPE_RESPONSE, &pb.Message_Response{response}}
	bodyBuffer, err := proto.Marshal(msg)

	if err != nil {
		c.errorClose(err)
		return
	}

	c.writeBuffer(bodyBuffer)
}

func (c *Connection) writeBuffer(msgBuffer []byte) {
	c.writeCh <- msgBuffer
}

func (c *Connection) handleResponse(msg *pb.Message, response *pb.Response) {
	c.waitMutex.Lock()
	waitCh, ok := c.wait[response.For]

	if !ok {
		c.waitMutex.Unlock()
		return
	}

	delete(c.wait, response.For)
	c.waitMutex.Unlock()

	if response.Error {
		waitCh <- &waitResponse{RemoteError, nil}
	} else {
		waitCh <- &waitResponse{nil, response.Body}
	}

	if c.closing {
		c.waitMutex.RLock()
		size := len(c.wait)
		c.waitMutex.RUnlock()

		if size == 0 {
			c.close()
		}
	}
}

func (c *Connection) startWriteLoop() {
	for msgBuffer := range c.writeCh {
		bodyBufferLen := len(msgBuffer)

		sizeBuffer := make([]byte, 4)
		binary.BigEndian.PutUint32(sizeBuffer, uint32(bodyBufferLen))

		finalBufferSize := 4 + bodyBufferLen
		buffer := make([]byte, finalBufferSize)

		copy(buffer, sizeBuffer)
		copy(buffer[4:], msgBuffer)

		needWriteLen := finalBufferSize
		allWrittenLen := 0
		sleepCount := 0

		for {
			writtenSize, err := c.con.Write(buffer[allWrittenLen:])

			if err != nil {
				c.errorClose(err)
			}

			allWrittenLen += writtenSize

			if allWrittenLen == needWriteLen {
				break
			}

			if sleepCount == 5 {
				c.errorClose(WriteTimeout)
				return
			}

			sleepCount++
			time.Sleep(200)
		}
	}
}

func (c *Connection) setCloseHandler(handler closeHandler) {
	c.closeHandler = handler
}
