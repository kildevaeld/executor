package socket

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestWriteHeader(t *testing.T) {
	writer := bytes.NewBuffer(nil)

	err := writeHeader(writer, 2, 100, ResponseMsg)

	if err != nil {
		t.Error(err)
	}

	b := writer.Bytes()

	if len(b) != 7 {
		t.Errorf("expected header length to be 7 was %d", len(b))
	}

	mt := MessageType(b[0])
	if mt != ResponseMsg {
		t.Errorf("expected message type to b %d was %d", ResponseMsg, mt)
	}

	id := binary.LittleEndian.Uint16(b[1:])
	if id != uint16(2) {
		t.Errorf("expected id to be 2 was %d", id)
	}

	ln := binary.LittleEndian.Uint32(b[3:])
	if ln != uint32(100) {
		t.Errorf("expected length to be 2 was %d", ln)
	}

}

func TestReadHeader(t *testing.T) {

	bs := []byte{2, 2, 0, 100, 0, 0, 0}

	reader := bytes.NewBuffer(nil)
	reader.Write(bs)

	id, length, msgt, err := readHeader(reader)

	if err != nil {
		t.Error(err)
	}

	if msgt != ResponseMsg {
		t.Errorf("expected message type to b %d was %d", ResponseMsg, msgt)
	}

	if id != uint16(2) {
		t.Errorf("expected id to be 2 was %d", id)
	}

	if length != uint32(100) {
		t.Errorf("expected length to be 100 was %d", length)
	}

}

/*func TestWriteMessage(t *testing.T) {

	writer := bytes.NewBuffer(nil)

	msg := []byte("Hello, World")

	err := writeMessage(writer, 2, ResponseMsg, msg)

	if err != nil {
		t.Error(err)
	}

	i := binary.LittleEndian.Uint32(writer.Bytes())

	if int(i) != len(msg) {
		t.Errorf("inspected length to be: %d, was: %d", len(msg), i)
	}

	if !bytes.Equal(msg, writer.Bytes()[4:]) {
		t.Errorf("msg body no equal")
	}

}*/
