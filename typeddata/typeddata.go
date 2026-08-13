package typeddata

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"unsafe"

	"github.com/AndreiSec/haproxy-spoe-go/varint"
)

const (
	// TypeNull const for TypedData type
	TypeNull byte = 0
	// TypeBoolean const for TypedData type
	TypeBoolean byte = 1
	// TypeInt32 const for TypedData type
	TypeInt32 byte = 2
	// TypeUInt32 const for TypedData type
	TypeUInt32 byte = 3
	// TypeInt64 const for TypedData type
	TypeInt64 byte = 4
	// TypeUInt64 const for TypedData type
	TypeUInt64 byte = 5
	// TypeIPv4 const for TypedData type
	TypeIPv4 byte = 6
	// TypeIPv6 const for TypedData type
	TypeIPv6 byte = 7
	// TypeString const for TypedData type
	TypeString byte = 8
	// TypeBinary const for TypedData type
	TypeBinary byte = 9
)

// ErrEmptyBuffer describe error, if passed empty buffer for decoding
var ErrEmptyBuffer = errors.New("empty buffer for decode")

// ErrDecodingBufferTooSmall describe error for too small decoding buffer
var ErrDecodingBufferTooSmall = errors.New("decoding buffer too small")

// maxVarintLen is the largest number of bytes a uint64 takes in the SPOE varint
// encoding: one byte plus seven bits per following byte for the remaining bits.
const maxVarintLen = 10

// uvarint wraps varint.Uvarint, turning its -1 truncation signal into an error
// so callers never slice with a negative length.
func uvarint(buf []byte) (uint64, int, error) {
	v, n := varint.Uvarint(buf)
	if n < 0 {
		return 0, 0, ErrDecodingBufferTooSmall
	}
	return v, n, nil
}

// bytesToString returns a string sharing storage with b. The caller must not
// modify b or use the result after the storage backing b is reused.
func bytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return *(*string)(unsafe.Pointer(&b))
}

// Encode variable to TypedData value
// The value is appended to buf, which may be a buffer reused between calls.
// returns filled buffer, count of bytes and error
func Encode(data interface{}, buf []byte) ([]byte, int, error) {
	var n int

	// Stack scratch space for the varint prefixes, so encoding a value does not
	// allocate.
	var b [maxVarintLen]byte

	switch v := data.(type) {
	case nil:
		buf = append(buf, TypeNull)
		return buf, 1, nil

	case bool:
		var t byte = 0x11
		if !v {
			t = 0x01
		}
		buf = append(buf, t)
		return buf, 1, nil

	case int32:
		buf = append(buf, TypeInt32)
		i := varint.PutUvarint(b[:], uint64(v))
		buf = append(buf, b[:i]...)
		return buf, i + 1, nil

	case uint32:
		buf = append(buf, TypeUInt32)
		i := varint.PutUvarint(b[:], uint64(v))
		buf = append(buf, b[:i]...)
		return buf, i + 1, nil

	case int:
		buf = append(buf, TypeInt64)
		i := varint.PutUvarint(b[:], uint64(v))
		buf = append(buf, b[:i]...)
		return buf, i + 1, nil

	case int64:
		buf = append(buf, TypeInt64)
		i := varint.PutUvarint(b[:], uint64(v))
		buf = append(buf, b[:i]...)
		return buf, i + 1, nil

	case uint:
		buf = append(buf, TypeUInt64)
		i := varint.PutUvarint(b[:], uint64(v))
		buf = append(buf, b[:i]...)
		return buf, i + 1, nil

	case uint64:
		buf = append(buf, TypeUInt64)
		i := varint.PutUvarint(b[:], v)
		buf = append(buf, b[:i]...)
		return buf, i + 1, nil

	case string:
		n = 1
		buf = append(buf, TypeString)
		i := varint.PutUvarint(b[:], uint64(len(v)))
		n += i
		n += len(v)
		buf = append(buf, b[:i]...)
		buf = append(buf, v...)
		return buf, n, nil

	case []byte:
		n = 1
		buf = append(buf, TypeBinary)
		i := varint.PutUvarint(b[:], uint64(len(v)))
		n += i
		n += len(v)
		buf = append(buf, b[:i]...)
		buf = append(buf, v...)
		return buf, n, nil
	}

	// buf is returned untouched so callers appending in place keep what they
	// have already encoded.
	return buf, 0, fmt.Errorf("type not supported for encode to TypedData: %s", reflect.TypeOf(data).String())
}

// Options tunes decoding of TypedData values
type Options struct {
	// NoCopyStrings makes decoded string values alias the source buffer instead
	// of being copied onto the heap, which removes one allocation per string
	// argument. Values decoded this way are only valid for as long as the
	// buffer they were decoded from is: for frames read by a worker, that means
	// until the request handler returns. Binary and IP values always alias the
	// source buffer, regardless of this option.
	NoCopyStrings bool
}

// Decode TypedData value
// Returns decoded variable, bytes count and error
func Decode(buf []byte) (data interface{}, n int, err error) {
	return DecodeOpts(buf, Options{})
}

// DecodeOpts decodes a TypedData value with the given options
// Returns decoded variable, bytes count and error
func DecodeOpts(buf []byte, opts Options) (data interface{}, n int, err error) {
	if len(buf) == 0 {
		err = ErrEmptyBuffer
		return
	}

	f := buf[0] >> 4
	t := buf[0] & 0x0F
	buf = buf[1:]
	n = 1

	switch t {
	case TypeNull:
		return

	case TypeBoolean:
		data = f&0x01 > 0
		return

	case TypeInt32:
		i, l, e := uvarint(buf)
		if e != nil {
			return nil, n, e
		}
		n += l
		data = int32(i)
		return

	case TypeUInt32:
		i, l, e := uvarint(buf)
		if e != nil {
			return nil, n, e
		}
		n += l
		data = uint32(i)
		return

	case TypeInt64:
		i, l, e := uvarint(buf)
		if e != nil {
			return nil, n, e
		}
		n += l
		data = int64(i)
		return

	case TypeUInt64:
		i, l, e := uvarint(buf)
		if e != nil {
			return nil, n, e
		}
		n += l
		data = uint64(i)
		return

	case TypeIPv4:
		if len(buf) < 4 {
			err = ErrDecodingBufferTooSmall
			return
		}
		data = net.IP(buf[:4])
		n += 4
		return

	case TypeIPv6:
		if len(buf) < 16 {
			err = ErrDecodingBufferTooSmall
			return
		}
		data = net.IP(buf[:16])
		n += 16
		return

	case TypeString:
		sLen, i, e := uvarint(buf)
		if e != nil {
			return nil, n, e
		}
		n += i
		buf = buf[i:]
		if uint64(len(buf)) < sLen {
			err = ErrDecodingBufferTooSmall
			return
		}
		if opts.NoCopyStrings {
			data = bytesToString(buf[:sLen])
		} else {
			data = string(buf[:sLen])
		}
		n += int(sLen)
		return

	case TypeBinary:
		dataLen, i, e := uvarint(buf)
		if e != nil {
			return nil, n, e
		}
		n += i
		buf = buf[i:]
		if uint64(len(buf)) < dataLen {
			err = ErrDecodingBufferTooSmall
			return
		}
		data = buf[:dataLen]
		n += int(dataLen)
		return
	}

	return nil, n, fmt.Errorf("type %d not supported for decode from TypedData", t)
}
