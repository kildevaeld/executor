package socket

import (
	"net"

	"github.com/kildevaeld/executor"
)

type Client struct {
	channel  *Channel
	executor *executor.Executor
}

func (self *Client) Connect() error {

	conn, err := net.Dial("tcp", ":3000")

	if err != nil {
		return err
	}

	self.channel = NewChannel(getID(), self.executor, conn)

	err = self.channel.Handle()

	if err != nil {
		return err
	}

	self.channel.RequestServices()

	return nil
}

func (self *Client) GetServices() ServiceList {
	return self.channel.RequestServices()
}

func (self *Client) Call(method string, ctx interface{}, arg interface{}, reply interface{}) error {
	return self.channel.Call(method, ctx, arg, reply)
}

func NewClient() *Client {
	return &Client{
		executor: executor.NewExecutor(),
	}
}
