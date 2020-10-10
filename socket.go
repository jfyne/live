package live

import "nhooyr.io/websocket"

const (
	MaxMessageBufferSize = 16
)

// Socket describes a socket from the outside.
type Socket struct {
	Session Session
	Data    interface{}

	msgs      chan SocketMessage
	closeSlow func()
}

// SocketMessage messages that are sent and received by the
// socket.
type SocketMessage struct {
	T    Event       `json:"t"`
	Data interface{} `json:"d"`
}

// NewSocket creates a new socket.
func NewSocket(s Session) *Socket {
	return &Socket{
		Session: s,
		msgs:    make(chan SocketMessage, MaxMessageBufferSize),
	}
}

// AssignSocket to a socket.
func (c *Socket) AssignWS(ws *websocket.Conn) {
	c.closeSlow = func() {
		ws.Close(websocket.StatusPolicyViolation, "socket too slow to keep up with messages")
	}
}
