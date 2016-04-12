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

	conn, err := net.Dial("tcp", ":4000")

	if err != nil {
		return err
	}

	self.channel = NewChannel(getID(), self.executor, conn)

}
