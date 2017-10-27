package rpc

import (
	"log"
	"net"
)

type ServerRequestHandler func(*Connection, string, []byte) ([]byte, error)

func NewServer(requestHandler ServerRequestHandler) *Server {
	return &Server{
		nil,
		0,
		make(map[int]*Connection),
		requestHandler,
		nil,
		nil,
	}
}

type ConHandler func(*Connection)

type Server struct {
	server           net.Listener
	lastConnectionId int
	connections      map[int]*Connection
	requestHandler   ServerRequestHandler
	openHandler      ConHandler
	closeHandler     ConHandler
}

func (s *Server) Listen() error {
	lis, err := net.Listen("tcp", "localhost:9999")

	if err != nil {
		return err
	}

	s.server = lis
	return nil
}

func (s *Server) Serve() error {
	defer s.server.Close()

	for {
		con, err := s.server.Accept()

		if err != nil {
			log.Println("Accept error:", err)
			return err
		}

		s.lastConnectionId++
		connectionId := s.lastConnectionId

		var client *Connection = nil
		client = NewConnection(func(apiName string, params []byte) ([]byte, error) {
			return s.requestHandler(client, apiName, params)
		})
		client.Link(con)
		client.setCloseHandler(func() {
			delete(s.connections, connectionId)

			if s.closeHandler != nil {
				s.closeHandler(client)
			}
		})

		s.connections[connectionId] = client

		if s.openHandler != nil {
			s.openHandler(client)
		}
	}
}

func (s *Server) SetHandlers(open ConHandler, close ConHandler) {
	s.openHandler = open
	s.closeHandler = close
}
