package rpc

import (
	"log"
	"net"
)

func NewServer(requestHandler RequestHandler) *Server {
	return &Server{0, make(map[int]*Connection), requestHandler}
}

type Server struct {
	lastConnectionId int
	connections      map[int]*Connection
	requestHandler   RequestHandler
}

func (s *Server) Listen() {
	lis, err := net.Listen("tcp", "localhost:9999")

	if err != nil {
		log.Fatalln(err)
	}

	for {
		con, err := lis.Accept()

		if err != nil {
			log.Println("Accept error:", err)
			break
		}

		s.lastConnectionId++
		connectionId := s.lastConnectionId

		client := NewConnection(s.requestHandler)
		client.Link(con)
		client.setCloseHandler(func() {
			delete(s.connections, connectionId)
		})

		s.connections[connectionId] = client
	}
}
