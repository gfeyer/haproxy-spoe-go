package typeddata

import (
	"net"
	"testing"
)

func TestDecodeOpts_NoCopyStrings(t *testing.T) {
	buf := []byte{TypeString, 0x03, 'a', 'b', 'c'}

	v, n, err := DecodeOpts(buf, Options{NoCopyStrings: true})
	if err != nil {
		t.Fatal(err)
	}
	if n != len(buf) {
		t.Fatalf("read %d bytes, want %d", n, len(buf))
	}

	s, ok := v.(string)
	if !ok {
		t.Fatalf("got %T, want string", v)
	}
	if s != "abc" {
		t.Fatalf("got %q", s)
	}

	// The result aliases buf: that is the whole point, and the reason the option
	// is off by default.
	buf[2] = 'x'
	if s != "xbc" {
		t.Fatalf("expect result to alias the source buffer, got %q", s)
	}
}

func TestDecode_CopiesStringsByDefault(t *testing.T) {
	buf := []byte{TypeString, 0x03, 'a', 'b', 'c'}

	v, _, err := Decode(buf)
	if err != nil {
		t.Fatal(err)
	}

	buf[2] = 'x'
	if v.(string) != "abc" {
		t.Fatalf("expect a copy, got %q", v)
	}
}

func TestDecodeOpts_NoCopyEmptyString(t *testing.T) {
	v, n, err := DecodeOpts([]byte{TypeString, 0x00}, Options{NoCopyStrings: true})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("read %d bytes, want 2", n)
	}
	if v.(string) != "" {
		t.Fatalf("got %q", v)
	}
}

// TestDecode_Truncated feeds truncated encodings of every type and expects an
// error rather than a panic.
func TestDecode_Truncated(t *testing.T) {
	full := [][]byte{
		{TypeInt32, 0xf0},
		{TypeUInt32, 0xf0},
		{TypeInt64, 0xf0},
		{TypeUInt64, 0xf0},
		{TypeIPv4, 1, 2, 3},
		{TypeIPv6, 1, 2, 3, 4, 5, 6, 7, 8},
		{TypeString, 0x05, 'a', 'b'},
		{TypeString, 0xf0},
		{TypeBinary, 0x05, 'a', 'b'},
		{TypeBinary, 0xf0},
	}

	for _, buf := range full {
		if _, _, err := Decode(buf); err == nil {
			t.Fatalf("truncated value %x decoded without error", buf)
		}
		if _, _, err := DecodeOpts(buf, Options{NoCopyStrings: true}); err == nil {
			t.Fatalf("truncated value %x decoded without error", buf)
		}
	}
}

func TestDecode_IP(t *testing.T) {
	v, n, err := Decode([]byte{TypeIPv4, 192, 168, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("read %d bytes, want 5", n)
	}
	if !v.(net.IP).Equal(net.IPv4(192, 168, 0, 1)) {
		t.Fatalf("got %v", v)
	}
}

// TestEncode_LargeUint64 covers a value whose varint needs more than the eight
// byte scratch buffer the encoder used to allocate.
func TestEncode_LargeUint64(t *testing.T) {
	buf, n, err := Encode(uint64(1<<64-1), make([]byte, 0, 16))
	if err != nil {
		t.Fatal(err)
	}
	if n != len(buf) {
		t.Fatalf("reported %d bytes, appended %d", n, len(buf))
	}

	v, _, err := Decode(buf)
	if err != nil {
		t.Fatal(err)
	}
	if v.(uint64) != 1<<64-1 {
		t.Fatalf("got %v", v)
	}
}

func TestEncode_UnsupportedTypeKeepsBuf(t *testing.T) {
	buf := []byte{0xaa}

	got, n, err := Encode(struct{}{}, buf)
	if err == nil {
		t.Fatal("expect error for unsupported type")
	}
	if n != 0 {
		t.Fatalf("got n %d, want 0", n)
	}
	if len(got) != 1 || got[0] != 0xaa {
		t.Fatalf("expect buf returned untouched, got %x", got)
	}
}

func TestEncode_DoesNotAllocateScratch(t *testing.T) {
	buf := make([]byte, 0, 64)

	allocs := testing.AllocsPerRun(200, func() {
		buf, _, _ = Encode("value", buf[:0])
		buf, _, _ = Encode(uint32(12345), buf[:0])
		buf, _, _ = Encode(true, buf[:0])
	})

	if allocs != 0 {
		t.Fatalf("got %v allocs, want 0", allocs)
	}
}
