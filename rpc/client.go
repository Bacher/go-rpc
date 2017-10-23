package rpc

import (
	"net"
	"github.com/golang/protobuf/proto"
	"encoding/binary"
	"rpc/protocol"
	"sync"
	"log"
	"time"
	"errors"
)

type RequestHandler func(string, []byte) ([]byte, error)

type RequestResults struct {
	err error
	data []byte
}

var WriteTimeout = errors.New("socket write timeout")
var ResponseTimeout = errors.New("response timeout")
var ApiNotFound = errors.New("api not found")
var RemoteError = errors.New("remote error")
var Closed = errors.New("connection closed")
var NoHandler = errors.New("has not handler")

const REQUEST_TIMEOUT = 10 * time.Second

func NewClient(requestHandler RequestHandler) *Client {
	return &Client{
		nil,
		make(map[uint64]chan *waitResponse),
		0,
		&sync.Mutex{},
		false,
		false,
		false,
		requestHandler}
}

type waitResponse struct {
	err error
	data []byte
}

type Client struct {
	con net.Conn
	wait map[uint64]chan *waitResponse
	lastId uint64
	idMutex *sync.Mutex
	connected bool
	closing bool
	closed bool
	requestHandler RequestHandler
}

func (c *Client) Connect() error {
	con, err := net.Dial("tcp", "localhost:9999")

	if err != nil {
		return err
	}

	c.con = con
	c.connected = true

	go c.handle()

	return nil
}

func (c *Client) Link(con net.Conn) {
	c.con = con
	go c.handle()
}

func (c *Client) handle() {
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
			c.response(msg.Id, msg.GetRequest())
			break
		case pb.TYPE_RESPONSE:
			c.handleResponse(&msg, msg.GetResponse())
		}
	}
}

func (c *Client) Close() {
	if len(c.wait) == 0 {
		c.close()
	} else {
		c.closing = true
	}
}

func (c *Client) errorClose(err error) {
	c.close()
}

func (c *Client) close() {
	if !c.closed {
		c.closed = true

		for _, ch := range c.wait {
			ch <- &waitResponse{Closed, nil}
		}

		c.wait = nil

		c.con.Close()
		c.con = nil
	}
}

func (c *Client) getNextMessageId() uint64 {
	c.idMutex.Lock()
	c.lastId++
	id := c.lastId
	c.idMutex.Unlock()

	return id
}

func (c *Client) Request(apiName string, params proto.Message) ([]byte, error) {
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
	c.wait[id] = waitCh

	c.writeBuffer(msgBuffer)

	requestTimeout := time.NewTimer(REQUEST_TIMEOUT)

	select {
	case <-requestTimeout.C:
		delete(c.wait, id)
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

func (c *Client) response(requestId uint64, request *pb.Request) {
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
			log.Printf("[RPC] Request aborted. Too long response for \"%s\" [%ds]\n", request.Name, elapsed / time.Second)
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

func (c *Client) writeBuffer(msgBuffer []byte) {
	bodyBufferLen := len(msgBuffer)

	sizeBuffer := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBuffer, uint32(bodyBufferLen))

	finalBufferSize := 4 + bodyBufferLen;
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

func (c *Client) handleResponse(msg *pb.Message, response *pb.Response) {
	waitCh, ok := c.wait[response.For]

	if !ok {
		// TODO: delete
		log.Println("Receive response for unknown request", response)
		return
	}

	delete(c.wait, response.For)

	if response.Error {
		waitCh <- &waitResponse{RemoteError, nil}
	} else {
		waitCh <- &waitResponse{nil, response.Body}
	}

	if c.closing && len(c.wait) == 0 {
		c.close()
	}
}
