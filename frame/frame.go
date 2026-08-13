package frame

import (
	"sync"
	"sync/atomic"

	"github.com/AndreiSec/haproxy-spoe-go/action"
	"github.com/AndreiSec/haproxy-spoe-go/message"
	"github.com/AndreiSec/haproxy-spoe-go/payload/kv"
)

type Type byte

const (
	TypeUnset             Type = 0x00
	TypeHaproxyHello      Type = 0x01
	TypeHaproxyDisconnect Type = 0x02
	TypeNotify            Type = 0x03
	TypeAgentHello        Type = 0x65
	TypeAgentDisconnect   Type = 0x66
	TypeAgentAck          Type = 0x67
)

// MaxFrameLen bounds the frame length this package accepts, and therefore the
// largest buffer a single Read can allocate. HAProxy's negotiated max-frame-size
// is bounded by its own buffer size, tens of kilobytes in practice, so the
// default leaves ample headroom.
var MaxFrameLen uint32 = 16 << 20

// noCopyStrings holds the process wide policy set by SetNoCopyStrings
var noCopyStrings atomic.Bool

// SetNoCopyStrings controls whether string arguments decoded from a frame alias
// the frame's read buffer instead of being copied onto the heap. Enabling it
// removes one allocation per string argument, which is the bulk of what decoding
// a Notify frame allocates.
//
// It is off by default because it narrows the lifetime of decoded values: with it
// enabled, strings read from a Request are only valid until the handler returns,
// since the frame goes back to the pool and its buffer is reused. A handler that
// enables this must copy any string it stores, logs asynchronously, or otherwise
// keeps beyond its own return.
//
// Call it during startup, before any connection is served.
func SetNoCopyStrings(enabled bool) {
	noCopyStrings.Store(enabled)
}

// NoCopyStrings reports whether string arguments are decoded without copying.
func NoCopyStrings() bool {
	return noCopyStrings.Load()
}

var framePool = sync.Pool{
	New: func() interface{} {
		return NewFrame()
	},
}

func AcquireFrame() *Frame {
	return framePool.Get().(*Frame)
}

func ReleaseFrame(frame *Frame) {
	frame.Reset()
	framePool.Put(frame)
}

// Frame describe frame struct
type Frame struct {
	Len          uint32
	Type         Type
	Flags        uint32
	EngineID     string
	StreamID     uint64
	FrameID      uint64
	Healthcheck  bool
	MaxFrameSize uint32
	KV           *kv.KV
	Messages     *message.Messages
	Actions      action.Actions

	readBuf   []byte // reusable read buffer, retained across pool cycles
	encBuf    []byte // reusable encode buffer, retained across pool cycles
	tmp       [5]byte
	varintBuf [10]byte
}

// NewFrame creates and returns new Frame
// Fin byte in Flags already set
func NewFrame() *Frame {
	f := &Frame{
		Flags:    0x01,
		KV:       kv.AcquireKV(),
		Messages: message.NewMessages(),
	}

	return f
}

func (f *Frame) Reset() {
	f.Len = 0
	f.Type = 0
	f.Flags = 0x01
	f.EngineID = ""
	f.StreamID = 0
	f.FrameID = 0
	f.Healthcheck = false
	f.MaxFrameSize = 0

	f.Actions = nil
	f.Messages.Reset()
	f.KV.Reset()
}

// IsFin returns true, if frame has flag 'FIN'
func (f *Frame) IsFin() bool {
	return f.Flags&0x01 > 0
}

// IsAbort returns true, if frame has flag 'ABORT'
func (f *Frame) IsAbort() bool {
	return f.Flags&0x02 > 0
}
