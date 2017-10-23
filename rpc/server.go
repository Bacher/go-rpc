package rpc

import (
	"net"
	"log"
)

func NewServer(requestHandler RequestHandler) *Server {
	return &Server{nil, requestHandler}
}

type Server struct {
	clients []*Client
	requestHandler RequestHandler
}

func (s *Server) Listen() {
	lis, err := net.Listen("tcp","localhost:9999")

	if err != nil {
		log.Fatalln(err)
	}

	for {
		con, err := lis.Accept()

		if err != nil {
			log.Println("Accept error:", err)
			break
		}

		client := NewClient(s.requestHandler)
		client.Link(con)

		s.clients = append(s.clients, client)
	}
}