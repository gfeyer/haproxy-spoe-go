package frame

import (
	"bytes"
	"io"
	"testing"

	"github.com/AndreiSec/haproxy-spoe-go/action"
	"github.com/AndreiSec/haproxy-spoe-go/varint"
)

// repeatReader serves the same frame bytes over and over without allocating,
// so the benchmark measures only the decode path.
type repeatReader struct {
	data []byte
	pos  int
}

func (r *repeatReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		if r.pos == len(r.data) {
			r.pos = 0
		}
		c := copy(p[n:], r.data[r.pos:])
		r.pos += c
		n += c
	}
	return n, nil
}

// BenchmarkFrame_ReadPooled measures the steady state of the worker loop:
// AcquireFrame -> Read -> ReleaseFrame, which is how frames are actually used.
func BenchmarkFrame_ReadPooled(b *testing.B) {
	r := &repeatReader{data: testFrame}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f := AcquireFrame()
		if err := f.Read(r); err != nil {
			b.Fatal(err)
		}
		ReleaseFrame(f)
	}
}

// BenchmarkFrame_ReadPooledNoCopy is BenchmarkFrame_ReadPooled with string
// arguments decoded in place.
func BenchmarkFrame_ReadPooledNoCopy(b *testing.B) {
	SetNoCopyStrings(true)
	defer SetNoCopyStrings(false)

	r := &repeatReader{data: testFrame}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f := AcquireFrame()
		if err := f.Read(r); err != nil {
			b.Fatal(err)
		}
		ReleaseFrame(f)
	}
}

// BenchmarkFrame_ReadPooledParallel exercises the pools under contention.
func BenchmarkFrame_ReadPooledParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		r := &repeatReader{data: testFrame}
		for pb.Next() {
			f := AcquireFrame()
			if err := f.Read(r); err != nil {
				b.Fatal(err)
			}
			ReleaseFrame(f)
		}
	})
}

// BenchmarkFrame_ReadPooledLarge uses a frame with a large string argument,
// which is the common shape in production (headers, URLs, bodies).
func BenchmarkFrame_ReadPooledLarge(b *testing.B) {
	r := &repeatReader{data: largeTestFrame(4096)}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f := AcquireFrame()
		if err := f.Read(r); err != nil {
			b.Fatal(err)
		}
		ReleaseFrame(f)
	}
}

// BenchmarkFrame_ReadPooledLargeNoCopy is BenchmarkFrame_ReadPooledLarge with
// string arguments decoded in place.
func BenchmarkFrame_ReadPooledLargeNoCopy(b *testing.B) {
	SetNoCopyStrings(true)
	defer SetNoCopyStrings(false)

	r := &repeatReader{data: largeTestFrame(4096)}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f := AcquireFrame()
		if err := f.Read(r); err != nil {
			b.Fatal(err)
		}
		ReleaseFrame(f)
	}
}

// BenchmarkFrame_EncodePooled measures writing an Ack frame the way the worker
// does: a pooled frame, and actions owned by a pooled request.
func BenchmarkFrame_EncodePooled(b *testing.B) {
	actions := make(action.Actions, 0, 1)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		actions.Reset()
		actions.SetVar(action.ScopeRequest, "reputation", "good")

		f := AcquireFrame()
		f.Type = TypeAgentAck
		f.FrameID = 123
		f.StreamID = 456
		f.Actions = actions
		if _, err := f.Encode(io.Discard); err != nil {
			b.Fatal(err)
		}
		ReleaseFrame(f)
	}
}

// largeTestFrame builds a Notify frame carrying one message with a single
// string argument of the requested size.
func largeTestFrame(size int) []byte {
	payload := &bytes.Buffer{}
	payload.Write([]byte{0x00, 0x00, 0x00, 0x01}) // flags
	payload.Write([]byte{0xfe, 0x12})             // stream-id
	payload.Write([]byte{0x01})                   // frame-id

	name := "check-request"
	payload.WriteByte(byte(len(name)))
	payload.WriteString(name)
	payload.WriteByte(1) // nb args

	arg := "body"
	payload.WriteByte(byte(len(arg)))
	payload.WriteString(arg)

	payload.WriteByte(0x08) // TypeString
	var vb [10]byte
	n := varint.PutUvarint(vb[:], uint64(size))
	payload.Write(vb[:n])
	payload.Write(bytes.Repeat([]byte("x"), size))

	out := &bytes.Buffer{}
	var lb [4]byte
	l := payload.Len() + 1
	lb[0] = byte(l >> 24)
	lb[1] = byte(l >> 16)
	lb[2] = byte(l >> 8)
	lb[3] = byte(l)
	out.Write(lb[:])
	out.WriteByte(byte(TypeNotify))
	out.Write(payload.Bytes())

	return out.Bytes()
}
