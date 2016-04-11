package executor

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"
)

type Socket struct{}

type ClientCodec interface{}

// Call represents an active RPC.
type Call struct {
	ServiceMethod string      // The name of the service and method to call.
	Args          interface{} // The argument to the function (*struct).
	Reply         interface{} // The reply from the function (*struct).
	Error         error       // After completion, the error status.
	Done          chan *Call  // Strobes when call is complete.
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

type MethodType interface {
	ArgType() reflect.Type
	ReplyType() reflect.Type
	CtxType() reflect.Type
}

type funcType struct {
	sync.Mutex // protects counters
	fn         reflect.Value
	argType    reflect.Type
	ctxType    reflect.Type
	replyType  reflect.Type
	numCalls   uint
}

func (m *funcType) ArgType() reflect.Type {
	return m.argType
}

func (m *funcType) ReplyType() reflect.Type {
	return m.replyType
}

func (m *funcType) CtxType() reflect.Type {
	return m.ctxType
}

type funcService struct {
	name   string
	method map[string]*funcType
}

func (s *funcService) Call(sending *sync.Mutex, mType MethodType, ctxv, argv, replyv reflect.Value) error {
	m, ok := mType.(*funcType)
	if !ok {
		return errors.New("wrong type of message")
	}

	m.Lock()
	m.numCalls++
	m.Unlock()

	returnValues := m.fn.Call([]reflect.Value{argv, ctxv, replyv})

	errInter := returnValues[0].Interface()

	if errInter != nil {
		return errInter.(error)
	}

	return nil
}

func (s *funcService) Method(name string) MethodType {
	if m, ok := s.method[name]; ok {
		return m
	}
	return nil
}

func (s *funcService) MethodNames() (names []string) {
	for n, _ := range s.method {
		names = append(names, n)
	}
	return
}

type methodType struct {
	sync.Mutex // protects counters
	method     reflect.Method
	ctxType    reflect.Type
	argType    reflect.Type
	replyType  reflect.Type

	numCalls uint
}

func (m *methodType) ArgType() reflect.Type {
	return m.argType
}

func (m *methodType) ReplyType() reflect.Type {
	return m.replyType
}

func (m *methodType) CtxType() reflect.Type {
	return m.ctxType
}

type Service interface {
	Method(name string) MethodType
	Call(sending *sync.Mutex, mType MethodType, ctxv, argv, replyv reflect.Value) error
	MethodNames() []string
}

type service struct {
	name   string                 // name of service
	rcvr   reflect.Value          // receiver of methods for the service
	typ    reflect.Type           // type of the receiver
	method map[string]*methodType // registered methods
}

func (s *service) Call(sending *sync.Mutex, mType MethodType, ctxv, argv, replyv reflect.Value) error {

	m, ok := mType.(*methodType)

	if !ok {
		return fmt.Errorf("wrong type of message, was: %v", mType)
	}

	m.Lock()
	m.numCalls++
	m.Unlock()
	function := m.method.Func
	// Invoke the method, providing a new value for the reply.

	returnValues := function.Call([]reflect.Value{s.rcvr, ctxv, argv, replyv})
	// The return value for the method is an error.
	errInter := returnValues[0].Interface()

	if errInter != nil {
		return errInter.(error)
	}

	return nil
}

func (s *service) Method(name string) MethodType {
	if m, ok := s.method[name]; ok {
		return m
	}
	return nil
}

func (s *service) MethodNames() (names []string) {
	for n, _ := range s.method {
		names = append(names, n)
	}

	return
}
