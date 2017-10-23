package main

import (
	"log"
	"rpc/protocol"
	"rpc/rpc"
	"time"
)

func main() {
	client := rpc.NewClient(nil)

	err := client.Connect()

	if err != nil {
		log.Fatalln(err)
	}

	defer client.Close()

	time.Sleep(10 * time.Minute)

	res, err := client.Request("kek", &pb.Params1{uint32(3), uint32(5)})

	if err != nil {
		log.Println("[TEST] Request failed!", err)
	} else {
		log.Println("[TEST] Result:", string(res))
	}

	time.Sleep(10 * time.Minute)
}
