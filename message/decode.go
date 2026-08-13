package message

import (
	"fmt"

	"github.com/AndreiSec/haproxy-spoe-go/internal/intern"
	"github.com/AndreiSec/haproxy-spoe-go/typeddata"
	"github.com/AndreiSec/haproxy-spoe-go/varint"
)

// Decode reads messages from buf until it is consumed. Decoded values do not
// reference buf.
func (m *Messages) Decode(buf []byte) error {
	return m.DecodeOpts(buf, typeddata.Options{})
}

// DecodeOpts reads messages from buf until it is consumed, with the given decode
// options.
func (m *Messages) DecodeOpts(buf []byte, opts typeddata.Options) error {
	for len(buf) > 0 {
		messageNameLen, n := varint.Uvarint(buf)
		if n < 0 {
			return fmt.Errorf("error decode message, truncated name length")
		}
		buf = buf[n:]

		// +1 for the argument count byte that follows the name
		if uint64(len(buf)) < messageNameLen+1 {
			return fmt.Errorf("error decode message, wrong buf len. Expect %d, got %d", messageNameLen+1, len(buf))
		}

		message := AcquireMessage()
		// Track the message right away, so a failure below still releases it
		// back to the pool when the caller resets.
		*m = append(*m, message)

		// Message names come from the HAProxy configuration and repeat on every
		// frame, so interning them keeps decoding allocation free.
		message.Name = intern.String(buf[:messageNameLen])
		buf = buf[messageNameLen:]

		nbArgs := int(buf[0])
		buf = buf[1:]

		n, err := message.KV.UnmarshalNBOpts(buf, nbArgs, opts)
		if err != nil {
			return err
		}

		buf = buf[n:]
	}

	return nil
}
