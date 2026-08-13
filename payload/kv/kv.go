package kv

import (
	"fmt"
	"sync"

	"github.com/AndreiSec/haproxy-spoe-go/internal/intern"
	"github.com/AndreiSec/haproxy-spoe-go/typeddata"
	"github.com/AndreiSec/haproxy-spoe-go/varint"
)

const (
	// initialItems is the capacity a fresh KV starts with. A SPOE message
	// carries a handful of arguments, so this is usually enough to never regrow.
	initialItems = 8

	// maxVarintLen is the largest number of bytes a uint64 takes in the SPOE
	// varint encoding.
	maxVarintLen = 10
)

var kvPool = sync.Pool{
	New: func() interface{} {
		return NewKV()
	},
}

func AcquireKV() *KV {
	return kvPool.Get().(*KV)
}

func ReleaseKV(kv *KV) {
	kv.Reset()
	kvPool.Put(kv)
}

type Item struct {
	Name  string
	Value interface{}
}

type KV struct {
	m []Item

	// buf is the reusable encode buffer returned by Bytes
	buf []byte
}

func NewKV() *KV {
	kv := &KV{
		m: make([]Item, 0, initialItems),
	}

	return kv
}

func (kv *KV) Data() []Item {
	return kv.m
}

// Reset drops all items, keeping the backing array for reuse. Entries are
// zeroed so a released KV does not pin the strings and values it decoded.
func (kv *KV) Reset() {
	for i := range kv.m {
		kv.m[i] = Item{}
	}
	kv.m = kv.m[:0]
}

func (kv *KV) Add(key string, value interface{}) {
	kv.m = append(kv.m, Item{key, value})
}

func (kv *KV) Get(key string) (interface{}, bool) {
	for i := range kv.m {
		if kv.m[i].Name == key {
			return kv.m[i].Value, true
		}
	}

	return nil, false
}

// Bytes encodes the items into the KV's reusable buffer. The returned slice is
// only valid until the next call to Bytes or Reset.
func (kv *KV) Bytes() ([]byte, error) {
	kv.buf = kv.buf[:0]

	var vb [maxVarintLen]byte

	for i := range kv.m {
		item := &kv.m[i]

		n := varint.PutUvarint(vb[:], uint64(len(item.Name)))
		kv.buf = append(kv.buf, vb[:n]...)
		kv.buf = append(kv.buf, item.Name...)

		var err error
		kv.buf, _, err = typeddata.Encode(item.Value, kv.buf)
		if err != nil {
			return nil, err
		}
	}

	return kv.buf, nil
}

// Unmarshal decodes items from buf until it is consumed. Decoded values do not
// reference buf.
func (kv *KV) Unmarshal(buf []byte) error {
	return kv.UnmarshalOpts(buf, typeddata.Options{})
}

// UnmarshalOpts decodes items from buf until it is consumed, with the given
// decode options.
func (kv *KV) UnmarshalOpts(buf []byte, opts typeddata.Options) error {
	for len(buf) > 0 {
		n, err := kv.unmarshalItem(buf, opts)
		if err != nil {
			return err
		}
		buf = buf[n:]
	}

	return nil
}

// UnmarshalNB decodes count items from buf and returns the number of bytes
// consumed. Decoded values do not reference buf.
func (kv *KV) UnmarshalNB(buf []byte, count int) (int, error) {
	return kv.UnmarshalNBOpts(buf, count, typeddata.Options{})
}

// UnmarshalNBOpts decodes count items from buf with the given decode options and
// returns the number of bytes consumed.
func (kv *KV) UnmarshalNBOpts(buf []byte, count int, opts typeddata.Options) (int, error) {
	var readBytes int

	// Make room for the whole batch in one allocation. An item needs at least
	// two bytes on the wire, so a count beyond that is malformed and must not
	// drive an allocation.
	if count > 0 && count <= len(buf)/2 && cap(kv.m)-len(kv.m) < count {
		kv.grow(count)
	}

	for i := 0; i < count; i++ {
		if len(buf) == 0 {
			return readBytes, fmt.Errorf("buffer unexpectly end")
		}

		n, err := kv.unmarshalItem(buf, opts)
		if err != nil {
			return readBytes, err
		}

		buf = buf[n:]
		readBytes += n
	}

	return readBytes, nil
}

// unmarshalItem decodes a single name/value pair and returns its size in buf.
func (kv *KV) unmarshalItem(buf []byte, opts typeddata.Options) (int, error) {
	keyLen, n := varint.Uvarint(buf)
	if n < 0 {
		return 0, fmt.Errorf("error unmarshal KV, truncated key length")
	}
	buf = buf[n:]

	if uint64(len(buf)) < keyLen {
		return 0, fmt.Errorf("error unmarshal KV, wrong buf len. Expect %d, got %d", keyLen, len(buf))
	}

	// Keys come from the HAProxy configuration, so the same handful of names
	// arrive with every frame. Interning them keeps decoding allocation free.
	key := intern.String(buf[:keyLen])
	buf = buf[keyLen:]

	value, vn, err := typeddata.DecodeOpts(buf, opts)
	if err != nil {
		return 0, err
	}

	kv.m = append(kv.m, Item{key, value})

	return n + int(keyLen) + vn, nil
}

// grow makes room for n more items in a single allocation.
func (kv *KV) grow(n int) {
	m := make([]Item, len(kv.m), len(kv.m)+n)
	copy(m, kv.m)
	kv.m = m
}
