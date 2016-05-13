package executor

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/hashicorp/go-multierror"
	"github.com/kildevaeld/dict"
)

// Precompute the reflect type for error.  Can't use error directly
// because Typeof takes an empty interface value.  This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()
var typeOfBytes = reflect.TypeOf([]byte(nil))

// Is this an exported - upper case - name?
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// A method signature needs to be as follows: (arg1, arg2, *response) error
func checkMethod(method reflect.Method) (reflect.Type, reflect.Type, reflect.Type, error) {

	mtype := method.Type
	mname := method.Name

	// Method must be exported.
	if method.PkgPath != "" {
		return nil, nil, nil, errors.New("not exported")
	}

	// Method needs four ins: receiver, ctx, *args, *reply.
	if mtype.NumIn() != 4 {
		return nil, nil, nil, fmt.Errorf("method %s has wrong number of ins: %d", mname, mtype.NumIn())
	}

	// First arg need not be a pointer.
	ctxType := mtype.In(1)
	if !isExportedOrBuiltinType(ctxType) {
		return nil, nil, nil, fmt.Errorf("%s argument type not exported: %v", mname, ctxType)
	}

	// Second arg need not be a pointer.
	argType := mtype.In(2)
	if !isExportedOrBuiltinType(argType) {
		return nil, nil, nil, fmt.Errorf("%s argument type not exported: %v", mname, argType)
	}
	// Third arg must be a pointer.
	replyType := mtype.In(3)
	if replyType.Kind() != reflect.Ptr {
		return nil, nil, nil, fmt.Errorf("method %s reply type not a pointer: %v", mname, replyType)
	}
	// Reply type must be exported.
	if !isExportedOrBuiltinType(replyType) {
		return nil, nil, nil, fmt.Errorf("method %s reply type not exported: %v", mname, replyType)
	}
	// Method needs one out.
	if mtype.NumOut() != 1 {
		return nil, nil, nil, fmt.Errorf("method %s has wrong number of outs: %d", mname, mtype.NumOut())
	}
	// The return type of the method must be error.
	if returnType := mtype.Out(0); returnType != typeOfError {
		return nil, nil, nil, fmt.Errorf("method %s returns %s not error", mname, returnType.String())
	}

	return ctxType, argType, replyType, nil
}

// suitableMethods returns suitable Rpc methods of typ, it will report
// error using log if reportErr is true.
func suitableMethods(typ reflect.Type, reportErr bool) map[string]*methodType {
	methods := make(map[string]*methodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mname := method.Name

		ctxType, argType, replyType, err := checkMethod(method)

		if err != nil {
			if reportErr {
				log.Println(err.Error())
			}
			continue
		}

		methods[mname] = &methodType{method: method, ctxType: ctxType, argType: argType, replyType: replyType}
	}
	return methods
}

func getType(t reflect.Type) (dict.Map, error) {
	kind := t.Kind()

	if kind == reflect.Ptr {
		t = t.Elem()
		kind = t.Kind()
	}
	str := ""
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		str = "number"
	case reflect.String:
		str = "string"
	case reflect.Struct, reflect.Map:
		str = "object"
	case reflect.Bool:
		str = "boolean"
	case reflect.Array, reflect.Slice:
		str = "array"

	default:

		return nil, fmt.Errorf("wrong type: %v", t)

	}

	m := dict.NewMap()

	m["type"] = str

	if str == "object" {
		m["additionalProperties"] = false
		if t == reflect.TypeOf(time.Time{}) {
			m["type"] = "string"
			m["format"] = "date-time"
		} else if kind == reflect.Map {
			m["additionalProperties"] = true
		} else {
			ps := dict.NewMap()

			var required []string

			for m := 0; m < t.NumField(); m++ {
				field := t.Field(m)
				tt, e := getType(field.Type)
				if e != nil {
					return nil, e
				}
				ps[field.Name] = tt
				required = append(required, field.Name)
			}
			m["properties"] = ps
			m["title"] = t.Name()
			m["required"] = required
		}

	} else if str == "array" {

		if t == typeOfBytes {
			// byte array
			m["type"] = "string"
			m["format"] = "base64"
		} else {
			target := t.Elem()
			item, err := getType(target)
			if err != nil {
				return nil, err
			}

			m["items"] = []dict.Map{item}
		}

	}

	return m, nil

}

func getMethodSchema(name string, meth MethodType) dict.Map {
	out := dict.Map{
		"type":  "object",
		"title": name,
	}

	props := dict.NewMap()

	var result *multierror.Error
	var err error
	var m dict.Map

	if m, err = getType(meth.CtxType()); err != nil {
		result = multierror.Append(result, err)
	}
	props["context"] = m

	if m, err = getType(meth.ArgType()); err != nil {
		result = multierror.Append(result, err)
	}
	props["argument"] = m
	out["propeties"] = props
	out["required"] = []string{"argument", "context"}

	return out

}
