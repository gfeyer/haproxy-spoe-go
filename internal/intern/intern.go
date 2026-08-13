// Package intern deduplicates the short, repeating names carried by every SPOE
// frame - message names and argument keys - so that decoding them does not
// allocate a fresh string per frame.
//
// The set of names an agent sees is defined by the HAProxy configuration, so it
// is small and fixed after warmup. Lookups are lock free; the table is only
// mutated the first time a name is seen.
package intern

import (
	"sync"
	"sync/atomic"
)

const (
	// maxLen is the longest name that is worth caching. Anything longer is
	// unlikely to be a protocol name.
	maxLen = 128

	// maxEntries bounds the table so that a peer sending unique names cannot
	// grow it without limit. Once the cap is reached, callers simply get a
	// freshly allocated string, as they did before interning existed.
	maxEntries = 1024
)

var (
	mu    sync.Mutex
	table atomic.Pointer[map[string]string]
)

// String returns a string with the same contents as b. The result never aliases
// b, so it stays valid after the source buffer is reused.
func String(b []byte) string {
	if p := table.Load(); p != nil {
		m := *p
		// Indexing a map with string(b) does not copy b.
		if s, ok := m[string(b)]; ok {
			return s
		}
	}

	s := string(b)

	if len(b) > 0 && len(b) <= maxLen {
		store(s)
	}

	return s
}

// store adds s to the table using copy on write, keeping String allocation free
// on the hit path.
func store(s string) {
	mu.Lock()
	defer mu.Unlock()

	old := table.Load()

	var n int
	if old != nil {
		if _, ok := (*old)[s]; ok {
			return
		}
		n = len(*old)
	}

	if n >= maxEntries {
		return
	}

	next := make(map[string]string, n+1)
	if old != nil {
		for k, v := range *old {
			next[k] = v
		}
	}
	next[s] = s

	table.Store(&next)
}
