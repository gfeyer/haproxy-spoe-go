package kv

import (
	"testing"

	"github.com/AndreiSec/haproxy-spoe-go/typeddata"
)

// helloPayload is a KV payload with two string items
var helloPayload = []byte("\x12supported-versions\x08\x032.0\x0ccapabilities\x08\x10pipelining,async")

func TestKV_Unmarshal(t *testing.T) {
	kv := NewKV()

	if err := kv.Unmarshal(helloPayload); err != nil {
		t.Fatal(err)
	}

	v, ok := kv.Get("supported-versions")
	if !ok {
		t.Fatal("supported-versions not found")
	}
	if v.(string) != "2.0" {
		t.Fatalf("got %q", v)
	}

	v, ok = kv.Get("capabilities")
	if !ok {
		t.Fatal("capabilities not found")
	}
	if v.(string) != "pipelining,async" {
		t.Fatalf("got %q", v)
	}
}

// TestKV_ResetKeepsCapacity is the point of the whole exercise: a reset KV must
// keep its item buffer, otherwise every decode regrows it from nothing.
func TestKV_ResetKeepsCapacity(t *testing.T) {
	kv := NewKV()

	for i := 0; i < 32; i++ {
		kv.Add("key", "value")
	}
	c := cap(kv.m)

	kv.Reset()

	if len(kv.m) != 0 {
		t.Fatalf("expect no items, got %d", len(kv.m))
	}
	if cap(kv.m) != c {
		t.Fatalf("capacity dropped: %d -> %d", c, cap(kv.m))
	}
}

// TestKV_ResetClearsItems makes sure a pooled KV does not keep decoded values
// alive.
func TestKV_ResetClearsItems(t *testing.T) {
	kv := NewKV()
	kv.Add("key", "value")

	kv.Reset()

	items := kv.m[:1]
	if items[0].Name != "" || items[0].Value != nil {
		t.Fatalf("stale item retained: %+v", items[0])
	}
}

func TestKV_UnmarshalAllocs(t *testing.T) {
	kv := NewKV()

	// warm the item buffer and the intern table
	for i := 0; i < 4; i++ {
		if err := kv.Unmarshal(helloPayload); err != nil {
			t.Fatal(err)
		}
		kv.Reset()
	}

	got := testing.AllocsPerRun(200, func() {
		if err := kv.Unmarshal(helloPayload); err != nil {
			t.Fatal(err)
		}
		kv.Reset()
	})

	// two string values: one copy plus one interface box each
	if got > 4 {
		t.Fatalf("got %v allocs per unmarshal, want at most 4", got)
	}
}

func TestKV_UnmarshalNoCopyAllocs(t *testing.T) {
	kv := NewKV()
	opts := typeddata.Options{NoCopyStrings: true}

	for i := 0; i < 4; i++ {
		if err := kv.UnmarshalOpts(helloPayload, opts); err != nil {
			t.Fatal(err)
		}
		kv.Reset()
	}

	got := testing.AllocsPerRun(200, func() {
		if err := kv.UnmarshalOpts(helloPayload, opts); err != nil {
			t.Fatal(err)
		}
		kv.Reset()
	})

	// only the interface box for each of the two values
	if got > 2 {
		t.Fatalf("got %v allocs per unmarshal, want at most 2", got)
	}
}

// TestKV_UnmarshalTruncated feeds every prefix of a two item payload and expects
// an error rather than a panic. UnmarshalNB is used so that a prefix ending on an
// item boundary is still short by one item.
func TestKV_UnmarshalTruncated(t *testing.T) {
	for i := 0; i < len(helloPayload); i++ {
		kv := NewKV()
		if _, err := kv.UnmarshalNB(helloPayload[:i], 2); err == nil {
			t.Fatalf("truncated payload of %d bytes decoded without error", i)
		}
	}
}

func TestKV_UnmarshalNBCountMismatch(t *testing.T) {
	kv := NewKV()

	if _, err := kv.UnmarshalNB(helloPayload, 3); err == nil {
		t.Fatal("expect error when asking for more items than the buffer holds")
	}
}

func TestKV_Bytes(t *testing.T) {
	kv := NewKV()
	kv.Add("supported-versions", "2.0")
	kv.Add("capabilities", "pipelining,async")

	b, err := kv.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(helloPayload) {
		t.Fatalf("got %x, want %x", b, helloPayload)
	}

	// The buffer is reused, so a second call must not append to the first result
	b2, err := kv.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if len(b2) != len(helloPayload) {
		t.Fatalf("second call returned %d bytes, want %d", len(b2), len(helloPayload))
	}
}

func TestKV_BytesRoundTrip(t *testing.T) {
	out := NewKV()
	out.Add("healthcheck", true)
	out.Add("max-frame-size", uint32(16384))
	out.Add("engine-id", "abc")

	b, err := out.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	in := NewKV()
	if err = in.Unmarshal(b); err != nil {
		t.Fatal(err)
	}

	if v, _ := in.Get("healthcheck"); v != true {
		t.Fatalf("healthcheck: got %v", v)
	}
	if v, _ := in.Get("max-frame-size"); v != uint32(16384) {
		t.Fatalf("max-frame-size: got %v", v)
	}
	if v, _ := in.Get("engine-id"); v != "abc" {
		t.Fatalf("engine-id: got %v", v)
	}
}
