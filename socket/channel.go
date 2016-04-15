package socket

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

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
	id            MsgId
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
			return &Call{
				Done: make(chan *Call, 10),
			}
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
	services  ServiceList
}

/*func (self *Channel) Id() MsgId {
	return self.id
}*/

func (self *Channel) Errors() <-chan error {
	return self.errorChan
}

func (self *Channel) Handle() error {

	go self.handleRead()

	return nil
}

func (self *Channel) RequestServices() ServiceList {

	call, id := self.getCall()
	defer self.FreeCall(call)

	var reply []executor.ServiceDesc
	call.Reply = &reply
	call.Done = make(chan *Call, 100)

	self.send(call, id, SchemaRequestMsg, false)

	<-call.Done

	if call.Error != nil {
		self.errorChan <- wrapError("RequestService", call.Error)
	}

	self.lock.Lock()
	self.services.Append(reply...)
	//self.services = append(self.services, reply...)
	self.lock.Unlock()
	return self.services
}

func (self *Channel) HasService(method string) bool {

	dot := strings.LastIndex(method, ".")
	if dot < 0 {
		return false
	}
	self.lock.Lock()
	services := self.services
	self.lock.Unlock()

	for _, s := range services {
		if s.Name != method[:dot] {
			continue
		}
		for _, m := range s.Methods {
			if m.Name == method[dot+1:] {
				return true
			}
		}
	}

	return false
}

func (self *Channel) Go(method string, ctx interface{}, arg interface{}, reply interface{}, done chan *Call) *Call {

	call, id := self.getCall()
	call.ServiceMethod = method
	call.Ctx = ctx
	call.Arg = arg
	call.Reply = reply

	if done == nil {
		done = make(chan *Call, 10)
	} else {
		if cap(done) == 0 {
			log.Panicf("rpc: done channel is unbuffered")
		}
	}

	call.Done = done

	self.send(call, id, RequestMsg, true)
	return call
}

func (self *Channel) Call(method string, ctx interface{}, arg interface{}, reply interface{}) error {
	call := <-self.Go(method, ctx, arg, reply, nil).Done
	defer self.FreeCall(call)
	return call.Error
}

func (self *Channel) FreeCall(call *Call) {
	call.id = 0
	close(call.Done)
	callPool.Put(call)
	self.lock.Lock()
	defer self.lock.Unlock()
	delete(self.pending, call.id)
}

func (self *Channel) getCall() (*Call, MsgId) {
	call := callPool.Get().(*Call)
	id := getMsgID()
	self.lock.Lock()

	self.pending[id] = call
	call.id = id
	defer self.lock.Unlock()
	return call, id
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

func (self *Channel) send(call *Call, id MsgId, msgT MessageType, useDesc bool) {
	dot := strings.LastIndex(call.ServiceMethod, ".")

	clean := func() {
		time.Sleep(100 * time.Millisecond)
		call.done()
	}

	if dot < 0 && useDesc {
		call.Error = errors.New("invalid method name. Usage Service.Method")
		clean()
	} else {
		var err error
		if useDesc {
			desc := executor.CallDescription{
				Context:  call.Ctx,
				Argument: call.Arg,
				Service:  call.ServiceMethod[:dot],
				Method:   call.ServiceMethod[dot+1:],
			}

			err = self.writeStructMsg(id, msgT, desc)
		} else {
			err = writeMessage(self.conn, id, msgT, nil)
		}

		if err != nil {
			clean()
		}
	}

}

func NewChannel(id int, e *executor.Executor, conn net.Conn) *Channel {
	chann := &Channel{
		conn:     conn,
		id:       id,
		executor: e,
		pending:  make(map[MsgId]*Call),
	}

	return chann
}
