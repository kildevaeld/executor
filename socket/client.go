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

func NewClient() *Client {
	return &Client{
		executor: executor.NewExecutor(),
	}
}
