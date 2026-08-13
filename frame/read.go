package frame

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/AndreiSec/haproxy-spoe-go/typeddata"
	"github.com/AndreiSec/haproxy-spoe-go/varint"
)

// minReadBuf is the smallest payload buffer a frame keeps around. Rounding small
// frames up to it avoids reallocating when frame sizes vary slightly.
const minReadBuf = 1024

// Read decodes one frame from src into f.
//
// Decoded binary and IP values alias f's internal read buffer, which is reused by
// the next Read on this frame, so they must not be retained after the frame is
// released. The same holds for string values when NoCopyStrings is enabled.
func (f *Frame) Read(src io.Reader) error {
	var n int
	var err error

	n, err = io.ReadFull(src, f.tmp[:])
	if err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("error read frame size, %v", err)
	}

	f.Len = binary.BigEndian.Uint32(f.tmp[0:4])
	f.Type = Type(f.tmp[4])

	// Drop packet that doesn't have defined frame type early, before allocating any buffers
	// that way spurious connections (say someone calling curl on port) won't cause it to
	// allocate gigabytes of RAM
	switch f.Type {
	case TypeHaproxyHello, TypeHaproxyDisconnect, TypeNotify, TypeAgentHello, TypeAgentDisconnect, TypeAgentAck:
	default:
		return fmt.Errorf("unexpected frame type %d", f.Type)
	}

	// The length covers the type byte we already read, so anything below 1 is
	// malformed. Without this check the payload length underflows and we would
	// try to allocate 4GB from a five byte packet.
	if f.Len < 1 || f.Len > MaxFrameLen {
		return fmt.Errorf("unexpected frame length %d", f.Len)
	}

	payloadLen := int(f.Len - 1)
	f.growReadBuf(payloadLen)

	n, err = io.ReadFull(src, f.readBuf)
	if err != nil {
		return fmt.Errorf("error read frame, %v", err)
	}

	if n != payloadLen {
		return fmt.Errorf("unexpected frame length %d, expect %d", n, payloadLen)
	}

	buf := f.readBuf

	if len(buf) < 4 {
		return fmt.Errorf("unexpected frame length %d, expect at least 5", f.Len)
	}

	f.Flags = binary.BigEndian.Uint32(buf[0:4])
	buf = buf[4:]

	f.StreamID, n = varint.Uvarint(buf)
	if n < 0 {
		return fmt.Errorf("error read frame, truncated stream id")
	}
	buf = buf[n:]

	f.FrameID, n = varint.Uvarint(buf)
	if n < 0 {
		return fmt.Errorf("error read frame, truncated frame id")
	}
	buf = buf[n:]

	opts := typeddata.Options{NoCopyStrings: noCopyStrings.Load()}

	switch f.Type {
	case TypeHaproxyHello, TypeHaproxyDisconnect:
		// Hello and Disconnect values outlive the frame (EngineID is kept by the
		// worker), so they are always copied.
		if err = f.KV.Unmarshal(buf); err != nil {
			return err
		}
		if v, ok := f.KV.Get("healthcheck"); ok {
			if b, isBool := v.(bool); isBool {
				f.Healthcheck = b
			}
		}
		if v, ok := f.KV.Get("max-frame-size"); ok {
			if s, isUint32 := v.(uint32); isUint32 {
				f.MaxFrameSize = s
			}
		}
		if v, ok := f.KV.Get("engine-id"); ok {
			if s, isString := v.(string); isString {
				f.EngineID = s
			}
		}

	case TypeNotify:
		err = f.Messages.DecodeOpts(buf, opts)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("unexpected frame type %d", f.Type)
	}

	return nil
}

// growReadBuf sizes the reusable payload buffer to hold exactly n bytes, growing
// its capacity in rounded steps so that a stream of similarly sized frames
// allocates once instead of once per size.
func (f *Frame) growReadBuf(n int) {
	if cap(f.readBuf) < n {
		c := minReadBuf
		for c < n {
			c *= 2
		}
		f.readBuf = make([]byte, n, c)
		return
	}

	f.readBuf = f.readBuf[:n]
}
