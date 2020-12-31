package live

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"golang.org/x/net/html"
	"nhooyr.io/websocket"
)

const (
	MaxMessageBufferSize = 16
)

// Socket describes a socket from the outside.
type Socket struct {
	Session Session

	currentRender *html.Node
	msgs          chan Event
	closeSlow     func()

	data   interface{}
	dataMu sync.Mutex
}

// Assigns returns the data currently assigned to this
// socket.
func (s *Socket) Assigns() interface{} {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	return s.data
}

// Assign assigns data to this socket.
func (s *Socket) Assign(data interface{}) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	s.data = data
}

// Send an event to this socket.
func (s *Socket) Send(msg Event) {
	s.msgs <- msg
}

func (s *Socket) mount(ctx context.Context, h *Handler, r *http.Request, connected bool) error {
	// Mount handler.
	data, err := h.Mount(ctx, h, r, s, connected)
	if err != nil {
		return fmt.Errorf("mount error: %w", err)
	}
	s.Assign(data)
	return nil
}

// handleHandler takes a handler and runs a mount and render.
func (s *Socket) handleHandler(ctx context.Context, h *Handler) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	// Render handler.
	output, err := h.Render(ctx, h.t, s.data)
	if err != nil {
		return fmt.Errorf("render error: %w", err)
	}
	node, err := html.Parse(output)
	if err != nil {
		return fmt.Errorf("html parse error: %w", err)
	}

	// Get diff
	if s.currentRender != nil {
		patches, err := Diff(s.currentRender, node)
		if err != nil {
			return fmt.Errorf("diff error: %w", err)
		}
		if len(patches) != 0 {
			msg := Event{
				T:    EventPatch,
				Data: patches,
			}
			s.msgs <- msg
		}
	}
	s.currentRender = node

	return nil
}

// NewSocket creates a new socket.
func NewSocket(s Session) *Socket {
	return &Socket{
		Session: s,
		msgs:    make(chan Event, MaxMessageBufferSize),
	}
}

// assignWS connect a web socket to a socket.
func (c *Socket) assignWS(ws *websocket.Conn) {
	c.closeSlow = func() {
		ws.Close(websocket.StatusPolicyViolation, "socket too slow to keep up with messages")
	}
}
