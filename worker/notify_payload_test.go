package worker

import (
	"bufio"
	"io"
	"net"
	"testing"
	"time"

	"github.com/AndreiSec/haproxy-spoe-go/action"
	"github.com/AndreiSec/haproxy-spoe-go/frame"
	"github.com/AndreiSec/haproxy-spoe-go/logger"
	"github.com/AndreiSec/haproxy-spoe-go/request"
	"github.com/AndreiSec/haproxy-spoe-go/varint"
)

// notifyFrame builds a Notify frame with one message named "get-ip-reputation"
// carrying an IPv4 argument and a string argument.
func notifyFrame(streamID, frameID uint64, host string) []byte {
	var vb [10]byte

	payload := make([]byte, 0, 128)
	payload = append(payload, 0x00, 0x00, 0x00, 0x01) // flags: FIN

	n := varint.PutUvarint(vb[:], streamID)
	payload = append(payload, vb[:n]...)
	n = varint.PutUvarint(vb[:], frameID)
	payload = append(payload, vb[:n]...)

	name := "get-ip-reputation"
	payload = append(payload, byte(len(name)))
	payload = append(payload, name...)
	payload = append(payload, 2) // nb args

	payload = append(payload, byte(len("ip")))
	payload = append(payload, "ip"...)
	payload = append(payload, 0x06, 193, 200, 227, 222) // IPv4

	payload = append(payload, byte(len("host")))
	payload = append(payload, "host"...)
	payload = append(payload, 0x08)
	n = varint.PutUvarint(vb[:], uint64(len(host)))
	payload = append(payload, vb[:n]...)
	payload = append(payload, host...)

	out := make([]byte, 0, len(payload)+5)
	l := len(payload) + 1
	out = append(out, byte(l>>24), byte(l>>16), byte(l>>8), byte(l), byte(frame.TypeNotify))
	out = append(out, payload...)

	return out
}

// readAgentFrame reads one frame the agent sent. Frame.Read only decodes the
// payload of the frame types an agent receives, so it reports an unexpected type
// for agent frames; the header fields it fills in before that are what this test
// checks.
func readAgentFrame(t *testing.T, r io.Reader, want frame.Type) *frame.Frame {
	t.Helper()

	f := frame.AcquireFrame()
	err := f.Read(r)
	if f.Type != want {
		t.Fatalf("expect frame type %v, got %v (%v)", want, f.Type, err)
	}

	return f
}

func TestWorkerNotifyPayload(t *testing.T) {
	for _, noCopy := range []bool{false, true} {
		name := "copy"
		if noCopy {
			name = "nocopy"
		}

		t.Run(name, func(t *testing.T) {
			frame.SetNoCopyStrings(noCopy)
			defer frame.SetNoCopyStrings(false)

			clientConn, server := net.Pipe()
			defer clientConn.Close()

			type seen struct {
				host     string
				ip       net.IP
				engineID string
			}
			seenCh := make(chan seen, 4)

			handler := func(r *request.Request) {
				m, err := r.Messages.GetByName("get-ip-reputation")
				if err != nil {
					t.Errorf("message not found: %v", err)
					return
				}

				host, ok := m.KV.Get("host")
				if !ok {
					t.Error("host not found")
					return
				}
				ip, ok := m.KV.Get("ip")
				if !ok {
					t.Error("ip not found")
					return
				}

				// Copy out: with NoCopyStrings the value is only valid here
				s := host.(string)
				seenCh <- seen{
					host:     string([]byte(s)),
					ip:       append(net.IP(nil), ip.(net.IP)...),
					engineID: r.EngineID,
				}

				r.Actions.SetVar(action.ScopeSession, "ip_score", 42)
			}

			go Handle(server, handler, logger.NewNop())

			reader := bufio.NewReader(clientConn)

			hello := frame.AcquireFrame()
			hello.Type = frame.TypeHaproxyHello
			hello.KV.Add("supported-versions", "2")
			hello.KV.Add("max-frame-size", uint32(16*1024))
			hello.KV.Add("capabilities", "pipelining")
			hello.KV.Add("engine-id", "engine-1")
			sendFrame(t, clientConn, hello)
			frame.ReleaseFrame(hello)

			frame.ReleaseFrame(readAgentFrame(t, reader, frame.TypeAgentHello))

			// Several frames in a row, so that pooled frames and their reused
			// buffers are exercised.
			hosts := []string{"first.example.com", "second.example.com", "a-much-longer-host-name.example.com"}

			for i, host := range hosts {
				if _, err := clientConn.Write(notifyFrame(uint64(i+1), uint64(i+1), host)); err != nil {
					t.Fatalf("write notify: %v", err)
				}

				ack := readAgentFrame(t, reader, frame.TypeAgentAck)
				if ack.StreamID != uint64(i+1) || ack.FrameID != uint64(i+1) {
					t.Fatalf("wrong ack ids %d/%d", ack.StreamID, ack.FrameID)
				}
				frame.ReleaseFrame(ack)

				select {
				case got := <-seenCh:
					if got.host != host {
						t.Fatalf("handler saw host %q, want %q", got.host, host)
					}
					if got.ip.String() != "193.200.227.222" {
						t.Fatalf("handler saw ip %v", got.ip)
					}
					if got.engineID != "engine-1" {
						t.Fatalf("handler saw engine id %q", got.engineID)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("handler was not called")
				}
			}
		})
	}
}
