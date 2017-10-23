package rpc

import (
	"net"
	"log"
)

func NewServer() *Server {
	return &Server{nil}
}

type Server struct {
	clients []*Client
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

		client := NewClient()
		client.Link(con)

		s.clients = append(s.clients, client)
	}
}