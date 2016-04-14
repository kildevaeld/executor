package socket

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/kildevaeld/dict"
	"github.com/kildevaeld/executor"
)

var curMsgId MsgId = 0
var msgMutex sync.Mutex

func getMsgID() MsgId {
	msgMutex.Lock()
	defer msgMutex.Unlock()
	curMsgId++
	return curMsgId
}

func wrapError(msg string, err error) error {
	return fmt.Errorf("%s: %s", msg, err.Error())
}

type Call struct {
	ServiceMethod string
	Ctx           interface{}
	Arg           interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call
}

var callPool sync.Pool

func init() {
	callPool = sync.Pool{
		New: func() interface{} {
			return &Call{}
		},
	}
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
	pending   map[MsgId]*Call
	lock      sync.Mutex
}

/*func (self *Channel) Id() MsgId {
	return self.id
}*/

func (self *Channel) Errors() <-chan error {
	return self.errorChan
}

func (self *Channel) Handle() error {
	self.pending = make(map[MsgId]*Call)

	go self.handleRead()

	return nil
}

func (self *Channel) RequestServices() {
	call := callPool.Get().(*Call)

	call.Reply = &dict.Map{}
	call.Done = make(chan *Call, 100)
	id := getMsgID()

	self.lock.Lock()
	self.pending[id] = call
	self.lock.Unlock()

	writeMessage(self.conn, id, SchemaRequestMsg, nil)

	<-call.Done

	fmt.Printf("%#v", call.Reply)

	self.FreeCall(call)
}

func (self *Channel) FreeCall(call *Call) {
	callPool.Put(call)
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
		case ResponseMsg, SchemaResponseMsg:
			go self.handleResponse(msg, id)
		case SchemaRequestMsg:
			go self.handleMetaMessages(msgType, msg, id)
		case ErrorMsg:
			//self.handleError()
			//self.errorChan <- errors.New(string(msg))
		}

	}
}

func (self *Channel) handleError(id MsgId, err error) {
	if err := writeMessage(self.conn, id, ErrorMsg, []byte(err.Error())); err != nil {
		self.errorChan <- wrapError("ErrorWrite", err)
	}
}

func (self *Channel) handleRequest(msg []byte, id MsgId) {

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
		self.errorChan <- wrapError("RequestWrite", err)

	}

}

func (self *Channel) handleResponse(msg []byte, id MsgId) {

	self.lock.Lock()
	call := self.pending[id]
	delete(self.pending, id)
	self.lock.Unlock()

	if call == nil {
		self.handleError(id, fmt.Errorf("no call for %s", id))
		return
	}

	if err := json.Unmarshal(msg, &call.Reply); err != nil {
		call.Error = err
	}

	call.done()
}

func (self *Channel) writeStructMsg(id MsgId, msgT MessageType, v interface{}) error {

	bs, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return writeMessage(self.conn, id, msgT, bs)
}

func (self *Channel) handleMetaMessages(msgType MessageType, msg []byte, id MsgId) {

	switch msgType {
	case SchemaRequestMsg, SchemaResponseMsg:
		self.handleSchemaRequest(msgType, id)
	}
}

func (self *Channel) handleSchemaRequest(msgType MessageType, id MsgId) {

	if msgType == SchemaRequestMsg {

		bs, err := self.executor.GetServiceDescriptions()
		if err != nil {
			self.handleError(id, err)
			return
		}

		if err := writeMessage(self.conn, id, SchemaResponseMsg, bs); err != nil {
			self.errorChan <- wrapError("SchemeRequestWrite", err)
		}

	}

}

func (self *Channel) Call(method string, ctx interface{}, arg interface{}, reply interface{}) (*Call, error) {

	dot := strings.LastIndex(method, ".")

	if dot < 0 {
		return nil, errors.New("invalid method name. Usage Service.Method")
	}

	call := callPool.Get().(*Call)
	call.ServiceMethod = method
	call.Ctx = ctx
	call.Arg = arg
	call.Reply = reply
	call.Done = make(chan *Call, 100)
	id := getMsgID()
	self.lock.Lock()
	self.pending[id] = call
	self.lock.Unlock()

	desc := executor.CallDescription{
		Context:  ctx,
		Argument: ctx,
		Service:  method[:dot],
		Method:   method[dot+1:],
	}

	if err := self.writeStructMsg(id, RequestMsg, desc); err != nil {
		return nil, err
	}

	return call, nil
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
