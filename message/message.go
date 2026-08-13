package message

import (
	"sync"

	"github.com/AndreiSec/haproxy-spoe-go/payload/kv"
)

var messagePool = sync.Pool{
	New: func() interface{} {
		return newMessage()
	},
}

type Message struct {
	Name string
	KV   *kv.KV
}

func newMessage() *Message {
	// The KV is owned by the message for its whole lifetime, so it is created
	// directly instead of being borrowed from the KV pool.
	m := &Message{
		KV: kv.NewKV(),
	}

	return m
}

func AcquireMessage() *Message {
	m := messagePool.Get()
	if m == nil {
		return newMessage()
	}

	return m.(*Message)
}

func ReleaseMessage(m *Message) {
	m.Reset()
	messagePool.Put(m)
}

// Reset clears the message for reuse. The KV is kept rather than handed back to
// its pool: it belongs to this message for as long as the message is pooled, and
// keeping it preserves its already grown item buffer.
func (m *Message) Reset() {
	m.Name = ""

	m.KV.Reset()
}
