package rpc

import (
	"log"
	"time"
)

func NewClient(requestHandler RequestHandler) *Client {
	return &Client{nil, requestHandler}
}

type Client struct {
	con            *Connection
	requestHandler RequestHandler
}

func (c *Client) Connect() error {
	c.con = NewConnection(c.requestHandler)

	err := c.con.Connect()

	if err != nil {
		return err
	}

	c.con.setCloseHandler(func() {
		c.con = nil

		go func() {
			i := 0
			for {
				i++
				wait := time.Second

				if i > 60 {
					wait = time.Minute
				} else if i%3 == 0 {
					wait = 5 * time.Second
				}

				time.Sleep(wait)

				log.Println("Reconnecting...")
				err := c.Connect()

				if err == nil {
					break
				}
			}
		}()
	})

	return nil
}

func (c *Client) Request(apiName string, params []byte) ([]byte, error) {
	if c.con == nil {
		return nil, Closed
	} else {
		return c.con.Request(apiName, params)
	}
}

func (c *Client) Close() {
	if c.con != nil {
		c.con.Close()
	}
}
