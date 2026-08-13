package frame

import (
	"bytes"
	"testing"
)

// TestFrame_ReadRejectsZeroLength guards the underflow of Len-1: a five byte
// packet claiming a zero length used to ask for a 4GB buffer.
func TestFrame_ReadRejectsZeroLength(t *testing.T) {
	f := NewFrame()

	err := f.Read(bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, byte(TypeNotify)}))
	if err == nil {
		t.Fatal("expect error for zero length frame")
	}
	if cap(f.readBuf) != 0 {
		t.Fatalf("expect no buffer allocated, got cap %d", cap(f.readBuf))
	}
}

func TestFrame_ReadRejectsOversizedLength(t *testing.T) {
	f := NewFrame()

	err := f.Read(bytes.NewReader([]byte{0xff, 0xff, 0xff, 0xff, byte(TypeNotify)}))
	if err == nil {
		t.Fatal("expect error for oversized frame")
	}
	if cap(f.readBuf) != 0 {
		t.Fatalf("expect no buffer allocated, got cap %d", cap(f.readBuf))
	}
}

// TestFrame_ReadTruncated feeds every prefix of a valid frame's payload, with
// the declared length adjusted to match so parsing is actually reached. A short
// payload must be reported, never panic.
func TestFrame_ReadTruncated(t *testing.T) {
	for i := 5; i < len(testFrame); i++ {
		body := testFrame[5:i]

		frame := make([]byte, 0, len(body)+5)
		l := len(body) + 1
		frame = append(frame, byte(l>>24), byte(l>>16), byte(l>>8), byte(l), byte(TypeNotify))
		frame = append(frame, body...)

		f := NewFrame()
		err := f.Read(bytes.NewReader(frame))

		// A payload holding only flags and ids is a well formed frame with no
		// messages; anything that cuts into a message must be rejected.
		if err == nil && f.Messages.Len() == 0 && i > 12 {
			t.Fatalf("truncated frame of %d payload bytes decoded without error", len(body))
		}
	}
}

func TestFrame_ReadNoCopyStrings(t *testing.T) {
	SetNoCopyStrings(true)
	defer SetNoCopyStrings(false)

	f := NewFrame()
	if err := f.Read(bytes.NewReader(testFrame)); err != nil {
		t.Fatal(err)
	}

	messages := *f.Messages
	if len(messages) != 1 {
		t.Fatalf("expect 1 message, got %d", len(messages))
	}

	host, found := messages[0].KV.Get("host")
	if !found {
		t.Fatal("host not found")
	}
	if host.(string) != "domain.example.com" {
		t.Fatalf("wrong host %q", host)
	}
}

// TestFrame_ReadReusesBuffer checks the pooled read path stops allocating once
// the frame is warm.
func TestFrame_ReadReusesBuffer(t *testing.T) {
	f := NewFrame()

	for i := 0; i < 3; i++ {
		if err := f.Read(bytes.NewReader(testFrame)); err != nil {
			t.Fatal(err)
		}
		f.Reset()
	}

	before := cap(f.readBuf)
	if before == 0 {
		t.Fatal("expect read buffer to be retained")
	}

	if err := f.Read(bytes.NewReader(testFrame)); err != nil {
		t.Fatal(err)
	}
	if cap(f.readBuf) != before {
		t.Fatalf("read buffer reallocated: %d -> %d", before, cap(f.readBuf))
	}
}

func TestFrame_EncodeReuseAcrossFrames(t *testing.T) {
	f := NewFrame()
	f.Type = TypeAgentDisconnect
	f.KV.Add("status-code", uint32(0))
	f.KV.Add("message", "bye")

	first := &bytes.Buffer{}
	n, err := f.Encode(first)
	if err != nil {
		t.Fatal(err)
	}
	if n != first.Len() {
		t.Fatalf("returned %d bytes, wrote %d", n, first.Len())
	}

	// Encoding the same frame again must produce identical bytes, not append to
	// the reused buffer.
	second := &bytes.Buffer{}
	if _, err = f.Encode(second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("second encode differs:\n%x\n%x", first.Bytes(), second.Bytes())
	}
}

// TestFrame_EncodeDecodeRoundTrip walks a KV frame through Encode and Read.
func TestFrame_EncodeDecodeRoundTrip(t *testing.T) {
	out := NewFrame()
	out.Type = TypeHaproxyHello
	out.StreamID = 0
	out.FrameID = 0
	out.KV.Add("max-frame-size", uint32(16384))
	out.KV.Add("engine-id", "abc-123")
	out.KV.Add("healthcheck", true)

	buf := &bytes.Buffer{}
	if _, err := out.Encode(buf); err != nil {
		t.Fatal(err)
	}

	in := NewFrame()
	if err := in.Read(buf); err != nil {
		t.Fatal(err)
	}

	if in.MaxFrameSize != 16384 {
		t.Fatalf("wrong max frame size %d", in.MaxFrameSize)
	}
	if in.EngineID != "abc-123" {
		t.Fatalf("wrong engine id %q", in.EngineID)
	}
	if !in.Healthcheck {
		t.Fatal("expect healthcheck")
	}
}
