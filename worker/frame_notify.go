package worker

import (
	"fmt"

	"github.com/AndreiSec/haproxy-spoe-go/frame"
	"github.com/AndreiSec/haproxy-spoe-go/request"
)

func (w *worker) processNotifyFrame(f *frame.Frame) {
	defer frame.ReleaseFrame(f)
	defer w.wg.Done()

	req := request.AcquireRequest()
	defer request.ReleaseRequest(req)

	req.StreamID = f.StreamID
	req.FrameID = f.FrameID
	req.EngineID = w.engineID
	req.Messages = f.Messages

	w.handler(req)

	ackFrame := frame.AcquireFrame()
	defer frame.ReleaseFrame(ackFrame)

	ackFrame.Type = frame.TypeAgentAck
	ackFrame.StreamID = f.StreamID
	ackFrame.FrameID = f.FrameID
	ackFrame.Actions = req.Actions

	err := w.writeFrame(ackFrame)
	if err != nil {
		w.logger.Errorf("ack frame write failed: %v", err)
	}
}

// writeFrame encodes f straight to the connection. Encode buffers internally and
// issues a single Write, so no intermediate buffer is needed here.
func (w *worker) writeFrame(f *frame.Frame) error {
	if _, err := f.Encode(w.conn); err != nil {
		return fmt.Errorf("cannot write frame to connection: %w", err)
	}

	return nil
}
