package socket

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/kildevaeld/dict"
	"github.com/kildevaeld/executor"
)

type Call struct {
	ServiceMethod string
	Ctx           interface{}
	Reply         interface{}
	Error         error
	Done          chan<- *Call
}

func (call *Call) done() {
	select {
	case call.Done <- call:
		// ok
	default:
		// We don't want to block here.  It is the caller's responsibility to make
		// sure the channel has enough buffer space. See comment in Go().
		//if debugLog {
		log.Println("rpc: discarding Call reply due to insufficient Done chan capacity")
		//}
	}
}

type Channel struct {
	conn      net.Conn
	errorChan chan error
	executor  *executor.Executor
	id        int
	pending   map[uint16]*Call
	lock      sync.Mutex
}

/*func (self *Channel) Id() uint16 {
	return self.id
}*/

func (self *Channel) Errors() <-chan error {
	return self.errorChan
}

func (self *Channel) Handle() error {
	self.pending = make(map[uint16]*Call)

	go self.handleRead()

	return nil
}

func (self *Channel) handleRead() {
	for {

		msg, id, msgType, err := readMessage(self.conn)

		if err != nil {
			self.errorChan <- err
			continue
		}

		switch msgType {
		case RequestMsg:
			go self.handleRequest(msg, id)
		case ResponseMsg:
			go self.handleResponse(msg, id)
		case SchemaRequestMsg, SchemaResponsMsg:
			go self.handleMetaMessages(msgType, msg, id)
		}

	}
}

func (self *Channel) handleError(id uint16, err error) {

}

func (self *Channel) handleRequest(msg []byte, id uint16) {

	var out dict.Map
	if err := self.executor.CallWithJSON(msg, &out); err != nil {
		self.handleError(id, err)
		return
	}

	bs, err := json.Marshal(out)
	if err != nil {
		self.handleError(id, err)
		return
	}

	if err := writeMessage(self.conn, id, ResponseMsg, bs); err != nil {
		self.handleError(id, err)
	}

}

func (self *Channel) handleResponse(msg []byte, id uint16) {

	self.lock.Lock()
	call := self.pending[id]
	delete(self.pending, id)
	self.lock.Unlock()

	if call == nil {
		self.errorChan <- fmt.Errorf("no call for %s", id)
		return
	}

	if err := json.Unmarshal(msg, &call.Reply); err != nil {
		call.Error = err
	}

	call.done()
}

func (self *Channel) handleMetaMessages(msgType MessageType, msg []byte, id uint16) {

}

func NewChannel(id int, e *executor.Executor, conn net.Conn) *Channel {
	chann := &Channel{
		conn:     conn,
		id:       id,
		executor: e,
		//id:   getID(),
	}

	return chann
}
