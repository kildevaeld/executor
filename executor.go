package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/fatih/structs"
	"github.com/kildevaeld/dict"
	"github.com/xeipuuv/gojsonschema"
)

type MethodDesc struct {
	Name  string                 `json:"name"`
	Ctx   map[string]interface{} `json:"ctx"`
	Arg   map[string]interface{} `json:"arg"`
	Reply map[string]interface{} `json:"reply"`
}

type ServiceDesc struct {
	Name    string       `json:"name"`
	Methods []MethodDesc `json:"methods"`
}

type CallDescription struct {
	Service  string      `json:"service"`
	Method   string      `json:"method"`
	Context  interface{} `json:"context"`
	Argument interface{} `json:"argument"`
}

type Executor struct {
	services map[string]Service
	pending  map[uint64]*Call
	seq      uint64
	mutex    sync.RWMutex
}

func (self *Executor) Register(name string, v interface{}) error {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	s := new(service)
	s.typ = reflect.TypeOf(v)
	s.rcvr = reflect.ValueOf(v)

	if name == "" {
		name = reflect.Indirect(s.rcvr).Type().Name()
	}

	if name == "" {
		return errors.New("rpc.Register: no service name for type " + s.typ.String())
	}

	if _, ok := self.services[name]; ok {
		return fmt.Errorf("service '%s' already defined", name)
	}

	s.method = suitableMethods(s.typ, false)

	if len(s.method) == 0 {
		return nil
	}

	self.services[name] = s

	return nil
}

func (self *Executor) RegisterFunc(name string, v interface{}) *Executor {

	return self
}

func (self *Executor) Call(method string, ctx interface{}, args interface{}, ret interface{}) error {

	dot := strings.LastIndex(method, ".")

	if dot < 0 {
		return errors.New("invalid method name. Usage Service.Method")
	}

	self.mutex.RLock()
	service, ok := self.services[method[:dot]]
	self.mutex.RUnlock()

	if !ok {
		return fmt.Errorf("no service named: %s", method[:dot])
	}

	methodType := service.Method(method[dot+1:])

	if methodType == nil {
		return fmt.Errorf("no action service action: %s", method)
	}

	argv := reflect.ValueOf(args)
	retv := reflect.ValueOf(ret)
	ctxv := reflect.ValueOf(ctx)

	err := service.Call(nil, methodType, ctxv, argv, retv)

	if err != nil {
		return err
	}

	return nil
}

func (self *Executor) GetService(service string) Service {
	self.mutex.RLock()
	s := self.services[service]
	self.mutex.RUnlock()
	return s
}

func (self *Executor) CallWithJSON(bs []byte, v interface{}) error {

	var desc CallDescription

	if err := json.Unmarshal(bs, &desc); err != nil {
		return err
	}

	self.mutex.RLock()
	service, ok := self.services[desc.Service]
	self.mutex.RUnlock()

	if !ok {
		return fmt.Errorf("no service named: %s", desc.Service)
	}

	meth := service.Method(desc.Method)

	if meth == nil {
		return fmt.Errorf("service '%s' has no method: '%s'", desc.Service, desc.Method)
	}

	schema := GetMethodSchema(desc.Method, meth)

	d := dict.Map{
		"context":  desc.Context,
		"argument": desc.Argument,
	}

	loader := gojsonschema.NewGoLoader(schema)
	data := gojsonschema.NewGoLoader(d)

	r, e := gojsonschema.Validate(loader, data)

	if e != nil {
		return e
	}
	if !r.Valid() {
		return errors.New("Error")
	}

	var arg, ctx interface{}

	if arg, e = GetValue(meth.ArgType(), desc.Argument); e != nil {
		return e
	}

	if ctx, e = GetValue(meth.CtxType(), desc.Context); e != nil {
		return e
	}

	var out interface{}
	argv := reflect.ValueOf(arg).Elem()
	var retv reflect.Value

	switch v.(type) {
	case *dict.Map, *map[string]interface{}:
		out = reflect.New(meth.ReplyType().Elem()).Interface()
		retv = reflect.ValueOf(out) //.Elem()
	default:
		tt := reflect.TypeOf(v)
		if tt != meth.ReplyType() {
			return fmt.Errorf("Wrong return type was: %v, needs: %v", tt, meth.ReplyType())
		}
		retv = reflect.ValueOf(v)
	}

	ctxv := reflect.ValueOf(ctx).Elem()

	e = service.Call(nil, meth, ctxv, argv, retv)

	if e != nil {
		return e
	}

	if out != nil {

		getValue := func(isDict bool) interface{} {
			switch v := out.(type) {
			case *dict.Map:
				if isDict {
					return *v
				}
				return (*v).ToMap()
			case *map[string]interface{}:
				if isDict {
					return dict.Map(*v)
				}
				return *v
			default:
				if structs.IsStruct(out) {
					return structs.Map(v)
				}
			}
			return nil
		}

		switch m := v.(type) {
		case *dict.Map:
			*m = getValue(true).(dict.Map)
		case *map[string]interface{}:
			*m = getValue(false).(map[string]interface{})
		}

	}

	return e
}

func GetValue(v reflect.Type, value interface{}) (interface{}, error) {
	b, _ := json.Marshal(value)
	n := reflect.New(v).Interface()

	e := json.Unmarshal(b, n)
	if e != nil {
		return nil, e
	}
	return n, e
}

func (self *Executor) GetServiceDescriptions() ([]byte, error) {

	out, err := self.getDescriptions()

	if err != nil {
		return nil, err
	}

	return json.Marshal(out)

}

func (self *Executor) GetServiceDescriptions2() ([]ServiceDesc, error) {

	return self.getDescriptions()

}

func (self *Executor) getDescriptions() ([]ServiceDesc, error) {
	var out []ServiceDesc

	for name, _ := range self.services {

		desc, e := self.getDescription(name)
		if e != nil {
			return nil, e
		}
		out = append(out, desc)
	}

	return out, nil
}

func (self *Executor) GetDescription2(name string) (ServiceDesc, error) {
	return self.getDescription(name)
}

func (self *Executor) getDescription(name string) (ServiceDesc, error) {
	srv := self.services[name]

	if srv == nil {
		return ServiceDesc{}, errors.New("service does not exists")
	}

	out := ServiceDesc{
		Name: name,
	}

	for _, name := range srv.MethodNames() {

		m := srv.Method(name)
		var ctx, arg, reply dict.Map
		var err error
		if arg, err = getType(m.ArgType()); err != nil {
			return ServiceDesc{}, err
		}
		if ctx, err = getType(m.CtxType()); err != nil {
			return ServiceDesc{}, err
		}

		if reply, err = getType(m.ReplyType()); err != nil {
			return ServiceDesc{}, err
		}

		method := MethodDesc{
			Name:  name,
			Ctx:   ctx,
			Arg:   arg,
			Reply: reply,
		}

		out.Methods = append(out.Methods, method)

	}

	return out, nil
}

func (self *Executor) GetServiceDescription(name string) ([]byte, error) {
	out, err := self.getDescription(name)

	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(out, "", "  ")

}

func (self *Executor) GetSchema(name string) []byte {

	srv := self.services[name]

	if srv == nil {
		return nil
	}

	out := dict.NewMap()
	out["name"] = name
	methods := dict.NewMap()

	for _, name := range srv.MethodNames() {

		m := srv.Method(name)

		arg2, e2 := getType(m.ArgType())
		if e2 != nil {
			return nil
		}
		arg1, e1 := getType(m.CtxType())
		if e1 != nil {
			return nil
		}
		reply, er := getType(m.CtxType())
		if er != nil {
			return nil
		}

		args := dict.NewMap()

		args["arg1"] = arg1
		args["arg2"] = arg2
		args["reply"] = reply
		methods[name] = args

	}
	out["methods"] = methods

	b, _ := json.MarshalIndent(out, "", "  ")

	return b
}

func NewExecutor() *Executor {
	return &Executor{
		services: make(map[string]Service),
	}
}

func CallToJSON(method string, ctx interface{}, arg interface{}) ([]byte, error) {
	dot := strings.LastIndex(method, ".")

	if dot < 0 {
		return nil, errors.New("invalid method name. Usage Service.Method")
	}

	out := CallDescription{
		Service: method[:dot],
		Method:  method[dot+1:],
	}

	out.Context = ctx
	out.Argument = arg

	return json.Marshal(out)

}
