package rpc

import (
	"net"
	"github.com/golang/protobuf/proto"
	"encoding/binary"
	"rpc/protocol"
	"sync"
	"log"
	"io"
	"time"
	"errors"
	"fmt"
)

type RequestResults struct {
	err error
	data []byte
}

var WriteTimeout = errors.New("socket write timeout")
var ApiNotFound = errors.New("api not found")
var RemoteError = errors.New("remote error")

func NewClient() *Client {
	return &Client{nil, make(map[uint64]chan *waitResponse), 0, &sync.Mutex{}, func(apiName string, body []byte) ([]byte, error) {
		switch apiName {
		case "kek":
			var p pb.Params1;
			err := proto.Unmarshal(body, &p)
			if err != nil {
				return nil, err
			}
			result := method1(p)
			return proto.Marshal(result)
		}

		log.Printf("Unknown apiMethod for parse %s\n", apiName)
		return nil, ApiNotFound
	}}
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
	handleRequest func(string, []byte) ([]byte, error)
}

func (c *Client) Serve() {

}

func (c *Client) Connect() error {
	con, err := net.Dial("tcp", "localhost:9999")

	if err == nil {
		c.con = con
	}

	go c.handle()

	return err
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
			if err == io.EOF {
				log.Println("Connection closed")
			} else {
				log.Println("Read failed:", err)
			}
			return
		}

		packageSize := int(binary.BigEndian.Uint32(sizeBuffer))

		messageBuffer := make([]byte, packageSize)

		var receiveSize = 0

		for {
			curReceiveSize, err := c.con.Read(messageBuffer[receiveSize:])

			if err != nil {
				if err == io.EOF {
					log.Println("Connection closed")
				} else {
					log.Println("Read message failed:", err)
				}
				return
			}

			log.Println("Received:", curReceiveSize)

			receiveSize += curReceiveSize

			if receiveSize == packageSize {
				break
			}
		}

		log.Println("Exit from receive loop")

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
	c.con.Close()
}

func (c *Client) errorClose(err error) {
	log.Println("Connection error:", err)
	c.Close()
}

func (c *Client) getNextMessageId() uint64 {
	c.idMutex.Lock()
	c.lastId++
	id := c.lastId
	c.idMutex.Unlock()

	return id
}

func (c *Client) Request(apiName string, params proto.Message) ([]byte, error) {
	id := c.getNextMessageId()

	paramsBuffer, _ := proto.Marshal(params)

	msg := &pb.Message{id, pb.TYPE_REQUEST, &pb.Message_Request{&pb.Request{apiName, paramsBuffer}}}
	msgBuffer, err := proto.Marshal(msg)

	if err != nil {
		return nil, err
	}

	sizeBuffer := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBuffer, uint32(len(msgBuffer)))

	waitCh := make(chan *waitResponse)
	c.wait[id] = waitCh

	c.con.Write(sizeBuffer)
	c.con.Write(msgBuffer)

	response := <-waitCh

	if response.err != nil {
		return nil, response.err
	} else {
		return response.data, nil
	}
}

func (c *Client) response(requestId uint64, request *pb.Request) {
	resultBuffer, err := c.handleRequest(request.Name, request.Params)

	var response *pb.Response;

	if err != nil {
		response = &pb.Response{requestId, true, nil}
	} else {
		response = &pb.Response{requestId, false, resultBuffer}
	}

	msg := &pb.Message{c.getNextMessageId(), pb.TYPE_RESPONSE, &pb.Message_Response{response}}
	bodyBuffer, err := proto.Marshal(msg)
	bodyBufferLen := len(bodyBuffer)

	if err != nil {
		c.errorClose(err)
		return
	}

	sizeBuffer := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBuffer, uint32(bodyBufferLen))

	buffer := make([]byte, 4 + bodyBufferLen)

	copy(buffer, sizeBuffer)
	copy(buffer[4:], bodyBuffer)

	c.writeBuffer(buffer)
}

func (c *Client) writeBuffer(buffer []byte) {
	needWriteLen := len(buffer)
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
		log.Println("Receive response for unknown request", response)
		return
	}

	delete (c.wait, response.For)

	if response.Error {
		waitCh <- &waitResponse{RemoteError, nil}
	} else {
		waitCh <- &waitResponse{nil, response.Body}
	}
}

func method1(params1 pb.Params1) *pb.Result1 {
	return &pb.Result1{fmt.Sprintf("Gay %d", params1.A)}
}