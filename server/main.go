package main

import (
	"rpc/rpc"
)

func main() {
	server := rpc.NewServer()

	server.Listen()
}
