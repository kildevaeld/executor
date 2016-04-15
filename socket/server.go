package socket

import (
	"fmt"
	"net"
	"sync"

	"github.com/kildevaeld/executor"
)

var curId int = 0
var mutex sync.Mutex

func getID() int {
	mutex.Lock()
	defer mutex.Unlock()
	curId++
	return curId
}

type Message struct {
	Length  int
	Payload []byte
}

func (self *Message) Marshal() ([]byte, error) {
	return nil, nil
}

type Server struct {
	listener net.Listener
	executor *executor.Executor
	channels map[int]*Channel
}

func (self *Server) Exectutor() *executor.Executor {
	return self.executor
}

func (self *Server) Serve() error {

	l, e := net.Listen("tcp", ":3000")

	if e != nil {
		return e
	}

	self.listener = l

	self.channels = make(map[int]*Channel)
	for {

		conn, err := self.listener.Accept()

		if err != nil {

		} else {
			go self.handleConnection(conn)
		}

	}

	return nil
}

func (self *Server) handleConnection(conn net.Conn) {

	cn := NewChannel(getID(), self.executor, conn)
	self.channels[cn.id] = cn

	go cn.Handle()

	cn.RequestServices()

}

func (self *Server) Call(method string, ctx interface{}, arg interface{}, reply interface{}) error {

	channel := self.getService(method)

	if channel == nil {
		return fmt.Errorf("service not found")
	}

	return channel.Call(method, ctx, arg, reply)

}

func (self *Server) getService(name string) *Channel {
	//dot := strings.IndexOf(".")
	for _, c := range self.channels {
		if c.HasService(name) {
			return c
		}
	}
	return nil
}

func (self *Server) Register(name string, v interface{}) error {
	return self.executor.Register(name, v)

}

func NewServer() *Server {
	e := executor.NewExecutor()

	return &Server{
		executor: e,
	}

}
