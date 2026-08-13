package frame

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/AndreiSec/haproxy-spoe-go/varint"
)

// Encode writes the frame, length prefix included, to dest in a single Write and
// returns the number of bytes written.
func (f *Frame) Encode(dest io.Writer) (int, error) {
	buf, err := f.marshal()
	if err != nil {
		return 0, err
	}

	n, err := dest.Write(buf)
	if err != nil {
		return 0, fmt.Errorf("error write frame. writes %d, expect %d, err: %v", n, len(buf), err)
	}
	if n != len(buf) {
		return 0, fmt.Errorf("error write frame. writes %d, expect %d", n, len(buf))
	}

	return n, nil
}

// marshal builds the wire representation of the frame in its reusable encode
// buffer. The returned slice is only valid until the next call.
func (f *Frame) marshal() ([]byte, error) {
	buf := f.encBuf[:0]

	// Room for the length prefix, filled in once the body size is known
	buf = append(buf, 0, 0, 0, 0)

	buf = append(buf, byte(f.Type))

	binary.BigEndian.PutUint32(f.tmp[:], f.Flags)
	buf = append(buf, f.tmp[0:4]...)

	n := varint.PutUvarint(f.varintBuf[:], f.StreamID)
	buf = append(buf, f.varintBuf[:n]...)

	n = varint.PutUvarint(f.varintBuf[:], f.FrameID)
	buf = append(buf, f.varintBuf[:n]...)

	var err error

	switch f.Type {
	case TypeAgentHello, TypeAgentDisconnect, TypeHaproxyHello, TypeHaproxyDisconnect:
		var payload []byte
		if payload, err = f.KV.Bytes(); err == nil {
			buf = append(buf, payload...)
		}

	case TypeAgentAck:
		for i := range f.Actions {
			if buf, err = f.Actions[i].Marshal(buf); err != nil {
				break
			}
		}

	case TypeNotify:
		if len(*f.Messages) > 0 {
			err = fmt.Errorf("encoding Notify frame with Message isn't handled yet")
		}

	default:
		err = fmt.Errorf("unexpected frame type %d", f.Type)
	}

	// Keep the buffer, and whatever capacity it gained, for the next frame
	f.encBuf = buf

	if err != nil {
		return nil, err
	}

	binary.BigEndian.PutUint32(buf[0:4], uint32(len(buf)-4))

	return buf, nil
}
