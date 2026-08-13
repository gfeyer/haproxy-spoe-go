package intern

import (
	"strconv"
	"testing"
)

func TestString(t *testing.T) {
	b := []byte("engine-id")

	first := String(b)
	if first != "engine-id" {
		t.Fatalf("got %q", first)
	}

	// The result must not alias the input, otherwise reusing the read buffer
	// would corrupt cached names.
	b[0] = 'X'
	if first != "engine-id" {
		t.Fatalf("result aliases input: %q", first)
	}

	if second := String([]byte("engine-id")); second != "engine-id" {
		t.Fatalf("got %q", second)
	}
}

func TestStringHitDoesNotAllocate(t *testing.T) {
	b := []byte("frontend")
	String(b)

	if allocs := testing.AllocsPerRun(100, func() { _ = String(b) }); allocs != 0 {
		t.Fatalf("got %v allocs on the hit path, want 0", allocs)
	}
}

func TestStringEmpty(t *testing.T) {
	if s := String(nil); s != "" {
		t.Fatalf("got %q", s)
	}
}

// TestStringCapped checks that a peer sending unique names can't grow the table
// without limit; past the cap, callers just get a fresh string.
func TestStringCapped(t *testing.T) {
	for i := 0; i < maxEntries*2; i++ {
		want := "unique-name-" + strconv.Itoa(i)
		if got := String([]byte(want)); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}

	if n := len(*table.Load()); n > maxEntries {
		t.Fatalf("table grew to %d entries, cap is %d", n, maxEntries)
	}
}

func TestStringTooLongNotCached(t *testing.T) {
	long := make([]byte, maxLen+1)
	for i := range long {
		long[i] = 'a'
	}

	before := len(*table.Load())
	if got := String(long); len(got) != len(long) {
		t.Fatalf("got %d bytes, want %d", len(got), len(long))
	}
	if after := len(*table.Load()); after != before {
		t.Fatalf("oversized name was cached: %d -> %d", before, after)
	}
}

func BenchmarkString(b *testing.B) {
	key := []byte("authorization")
	String(key)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = String(key)
	}
}
