package message

import (
	"testing"

	"github.com/AndreiSec/haproxy-spoe-go/typeddata"
)

// notifyPayload holds one message named "check" with a single string argument
var notifyPayload = []byte("\x05check\x01\x04host\x08\x03abc")

func TestDecodeOpts_NoCopyStrings(t *testing.T) {
	buf := append([]byte(nil), notifyPayload...)

	mess := NewMessages()
	if err := mess.DecodeOpts(buf, typeddata.Options{NoCopyStrings: true}); err != nil {
		t.Fatal(err)
	}

	m, err := mess.GetByName("check")
	if err != nil {
		t.Fatal(err)
	}

	v, ok := m.KV.Get("host")
	if !ok {
		t.Fatal("host not found")
	}
	if v.(string) != "abc" {
		t.Fatalf("got %q", v)
	}

	// The message name must survive the buffer being reused even in no copy mode
	for i := range buf {
		buf[i] = 0
	}
	if m.Name != "check" {
		t.Fatalf("message name aliases the read buffer: %q", m.Name)
	}
}

// TestDecode_Truncated feeds every prefix of a valid payload and expects an error
// rather than a panic.
func TestDecode_Truncated(t *testing.T) {
	for i := 1; i < len(notifyPayload); i++ {
		mess := NewMessages()
		if err := mess.Decode(notifyPayload[:i]); err == nil {
			t.Fatalf("truncated payload of %d bytes decoded without error", i)
		}
		// a failed decode must still release its messages
		mess.Reset()
	}
}

// TestReset_KeepsKV covers the pooling change: a message keeps its KV across
// resets, so the KV's item buffer is reused instead of being handed back and
// regrown.
func TestReset_KeepsKV(t *testing.T) {
	m := AcquireMessage()
	kv := m.KV

	m.Name = "check"
	m.KV.Add("host", "abc")

	m.Reset()

	if m.KV != kv {
		t.Fatal("expect the message to keep its KV")
	}
	if m.Name != "" {
		t.Fatalf("expect empty name, got %q", m.Name)
	}
	if len(m.KV.Data()) != 0 {
		t.Fatalf("expect empty KV, got %d items", len(m.KV.Data()))
	}
}

func TestDecodeAllocs(t *testing.T) {
	mess := NewMessages()

	// warm the pools, the item buffers and the intern table
	for i := 0; i < 4; i++ {
		if err := mess.Decode(notifyPayload); err != nil {
			t.Fatal(err)
		}
		mess.Reset()
	}

	got := testing.AllocsPerRun(200, func() {
		if err := mess.Decode(notifyPayload); err != nil {
			t.Fatal(err)
		}
		mess.Reset()
	})

	// one string copy plus its interface box
	if got > 2 {
		t.Fatalf("got %v allocs per decode, want at most 2", got)
	}
}
