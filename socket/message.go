package socket

import (
	"encoding/binary"
	"errors"
	"io"
)

type MessageType uint8

const (
	RequestMsg MessageType = iota + 1
	ResponseMsg

	SchemaRequestMsg
	SchemaResponsMsg
)

func writeHeader(writer io.Writer, id uint16, length uint32, msgType MessageType) error {

	// Write id
	var idb [2]byte
	var lnb [4]byte
	//var mtb [1]byte
	var err error
	var w int
	w, err = writer.Write([]byte{uint8(msgType)})
	if err != nil || w != 1 {
		return errors.New("could not write msgtype")
	}

	binary.LittleEndian.PutUint16(idb[:], id)
	w, err = writer.Write(idb[:])
	if err != nil || w != 2 {
		return errors.New("could not write id")
	}

	binary.LittleEndian.PutUint32(lnb[:], length)
	w, err = writer.Write(lnb[:])
	if err != nil || w != 4 {
		return errors.New("could not write length")
	}

	return nil
}

func readHeader(reader io.Reader) (id uint16, length uint32, msgType MessageType, err error) {
	var idb [2]byte
	var lnb [4]byte
	var mtb [1]byte

	var r int

	r, err = reader.Read(mtb[:])
	if err != nil || r != 1 {
		return 0, 0, 0, errors.New("could not get msgtype of message")
	}
	msgType = MessageType(mtb[0])

	r, err = reader.Read(idb[:])
	if err != nil || r != 2 {
		return 0, 0, 0, errors.New("could not get id of message")
	}
	id = binary.LittleEndian.Uint16(idb[:])

	r, err = reader.Read(lnb[:])
	if err != nil || r != 4 {
		return 0, 0, 0, errors.New("could not get length of message")
	}
	length = binary.LittleEndian.Uint32(lnb[:])

	return
}

func readMessage(reader io.Reader) (msg []byte, id uint16, msgType MessageType, err error) {
	/*var idb [2]byte
	idr, ide := reader.Read(idb[:])

	if ide != nil || idr != 2 {
		return nil, 0, errors.New("could not get id of message")
	}

	id := binary.LittleEndian.Uint16(idb[:])

	var b [4]byte
	r, e := reader.Read(b[:])

	if e != nil || r != 4 {
		// handler error
		return nil, 0, errors.New("could no get length of message")
	}

	length := int(binary.LittleEndian.Uint32(b[:]))*/
	var length uint32
	id, length, msgType, err = readHeader(reader)

	if err != nil {
		return nil, 0, 0, err
	}

	msg = make([]byte, length)
	left := length
	for left > 0 {
		i := length - left
		r, e := reader.Read(msg[i:])

		if e != nil {
			return nil, 0, 0, e
		}

		left -= uint32(r)
	}
	/*var out executor.CallDescription
	err := json.Unmarshal(read[:], &out)

	if err != nil {
		return nil, err
	}*/

	return

}

func writeMessage(writer io.Writer, id uint16, msgType MessageType, bs []byte) error {
	length := len(bs)
	left := length

	/*var idb [2]byte
	binary.LittleEndian.PutUint16(idb[:], id)
	idw, ide := writer.Write(idb[:])

	if ide != nil {
		return ide
	}

	if idw != 2 {
		return errors.New("could not write id")
	}

	var l [4]byte
	binary.LittleEndian.PutUint32(l[:], uint32(length))

	w, err := writer.Write(l[:])

	if err != nil {
		return err
	}
	if w != 4 {
		return errors.New("could not write length")
	}*/

	if err := writeHeader(writer, id, uint32(length), msgType); err != nil {
		return err
	}

	for left > 0 {
		i := length - left
		w, err := writer.Write(bs[i:])

		if err != nil {
			return err
		}

		left -= w
	}

	return nil

}
