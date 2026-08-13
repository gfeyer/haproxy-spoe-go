package worker

import (
	"fmt"

	"github.com/AndreiSec/haproxy-spoe-go/frame"
)

func (w *worker) sendAgentDisconnect(f *frame.Frame, statusCode uint32, message string) error {
	agentDisconnectFrame := frame.AcquireFrame()
	defer frame.ReleaseFrame(agentDisconnectFrame)

	agentDisconnectFrame.Type = frame.TypeAgentDisconnect
	agentDisconnectFrame.FrameID = f.FrameID
	agentDisconnectFrame.StreamID = f.StreamID
	agentDisconnectFrame.KV.Add("status-code", statusCode)
	agentDisconnectFrame.KV.Add("message", message)

	if _, err := agentDisconnectFrame.Encode(w.conn); err != nil {
		return fmt.Errorf("error write to connection: %v", err)
	}

	return nil
}
