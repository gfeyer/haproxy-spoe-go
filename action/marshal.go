package action

import (
	"fmt"

	"github.com/AndreiSec/haproxy-spoe-go/typeddata"
	"github.com/AndreiSec/haproxy-spoe-go/varint"
)

// Marshal appends the wire representation of the action to buf and returns the
// result. On error buf is returned unchanged.
func (action *Action) Marshal(buf []byte) ([]byte, error) {
	var nb byte

	switch action.Type {
	case TypeSetVar:
		nb = nbVarsSetVar
	case TypeUnsetVar:
		nb = nbVarsUnsetVar
	default:
		return buf, fmt.Errorf("unexpected action type: %v", action.Type)
	}

	buf = append(buf, byte(action.Type), nb, byte(action.Scope))

	// Stack scratch space for the name length prefix
	var b [maxVarintLen]byte
	n := varint.PutUvarint(b[:], uint64(len(action.Name)))

	buf = append(buf, b[:n]...)
	buf = append(buf, action.Name...)

	// Encode straight into buf rather than into a throwaway slice
	buf, _, err := typeddata.Encode(action.Value, buf)
	if err != nil {
		return buf, err
	}

	return buf, nil
}
