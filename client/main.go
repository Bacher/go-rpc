package main

import (
	"log"
	"rpc/rpc"
	"rpc/protocol"
)

func main() {
	client := rpc.NewClient()

	err := client.Connect()

	if err != nil {
		log.Fatalln(err)
	}

	defer client.Close()

	res, err := client.Request("kek", &pb.Params1{uint32(3), uint32(5)})

	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Result:", string(res))
}
