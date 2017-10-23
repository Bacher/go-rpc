package main

import (
	"rpc/rpc"
	"rpc/protocol"
	"github.com/golang/protobuf/proto"
	"log"
	"fmt"
)

func main() {
	server := rpc.NewServer(func(apiName string, body []byte) ([]byte, error) {
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

	server.Listen()
}

func method1(params1 pb.Params1) *pb.Result1 {
	return &pb.Result1{fmt.Sprintf("Gay %d", params1.A)}
}