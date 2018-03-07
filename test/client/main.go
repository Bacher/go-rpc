package main

import (
	"github.com/golang/protobuf/proto"
	"gorpc/protocol"
	"gorpc/rpc"
	"log"
	"time"
)

func main() {
	client := rpc.NewClient("127.0.0.1:9999", func(apiName string, data []byte) ([]byte, error) {
		log.Fatalln("Kek")
		return nil, nil
	})

	err := client.Connect()

	if err != nil {
		log.Fatalln(err)
	}

	defer client.Close()

	for i := 0; i < 10; i++ {
		go func() {
			log.Println("Request!")

			buf, err := proto.Marshal(&pb.Params1{uint32(3), uint32(5)})

			if err != nil {
				log.Fatalln("Marshal failed:", err)
			}

			res, err := client.Request("kek", buf)

			if err != nil {
				log.Println("[TEST] Request failed!", err)
			} else {
				log.Println("[TEST] Result:", string(res))
			}
		}()
	}

	time.Sleep(3 * time.Second)
}
