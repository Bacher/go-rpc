package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"gorpc/protocol"
	"gorpc/rpc"
	"log"
	"time"
)

func main() {
	server := rpc.NewServer("127.0.0.1:9999", func(con *rpc.Connection, apiName string, body []byte) ([]byte, error) {
		switch apiName {
		case "kek":
			var p pb.Params1
			err := proto.Unmarshal(body, &p)
			if err != nil {
				return nil, err
			}
			result := method1(p)
			return proto.Marshal(result)
		}

		log.Printf("Unknown apiMethod for parse %s\n", apiName)
		return nil, rpc.ApiNotFound
	})

	if err := server.Listen(); err != nil {
		log.Println("Error")
	}

	if err := server.Serve(); err != nil {

	}
}

func method1(params1 pb.Params1) *pb.Result1 {
	time.Sleep(1 * time.Second)
	return &pb.Result1{fmt.Sprintf("Gay %d", params1.A)}
}
