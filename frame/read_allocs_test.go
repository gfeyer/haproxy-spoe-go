package frame

import (
	"testing"
)

// TestFrame_ReadAllocs pins down the allocation count of the hot decode path so
// that a regression shows up as a test failure.
//
// What remains after warmup is one allocation per decoded string value plus the
// interface{} box for each non trivial value; everything else - the payload
// buffer, the item slices, the keys and the message names - is reused or
// interned.
func TestFrame_ReadAllocs(t *testing.T) {
	const (
		// 1 string copy + 1 string box + 1 net.IP box
		wantCopy = 3
		// same, minus the string copy
		wantNoCopy = 2
	)

	for _, tc := range []struct {
		name   string
		noCopy bool
		want   float64
	}{
		{name: "copy", want: wantCopy},
		{name: "nocopy", noCopy: true, want: wantNoCopy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			SetNoCopyStrings(tc.noCopy)
			defer SetNoCopyStrings(false)

			r := &repeatReader{data: testFrame}
			f := AcquireFrame()
			defer ReleaseFrame(f)

			// warm the buffers and the intern table
			for i := 0; i < 4; i++ {
				if err := f.Read(r); err != nil {
					t.Fatal(err)
				}
				f.Reset()
			}

			got := testing.AllocsPerRun(200, func() {
				if err := f.Read(r); err != nil {
					t.Fatal(err)
				}
				f.Reset()
			})

			if got > tc.want {
				t.Fatalf("got %v allocs per read, want at most %v", got, tc.want)
			}
		})
	}
}

// TestFrame_EncodeAllocs covers the ack path, which should not allocate at all
// once the frame's encode buffer is warm.
func TestFrame_EncodeAllocs(t *testing.T) {
	f := AcquireFrame()
	defer ReleaseFrame(f)

	f.Type = TypeAgentDisconnect
	f.KV.Add("status-code", uint32(0))
	f.KV.Add("message", "connection closed by server")

	got := testing.AllocsPerRun(200, func() {
		if _, err := f.Encode(discardWriter{}); err != nil {
			t.Fatal(err)
		}
	})

	if got != 0 {
		t.Fatalf("got %v allocs per encode, want 0", got)
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
