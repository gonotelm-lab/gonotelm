package schema

const StreamHeartbeatPayload = "ping"

type StreamHeartbeat struct {
	Heartbeat string `json:"heartbeat"`
}

func NewStreamHeartbeat() *StreamHeartbeat {
	return &StreamHeartbeat{Heartbeat: StreamHeartbeatPayload}
}
