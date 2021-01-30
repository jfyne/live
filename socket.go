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
	// maxMessageBufferSize the maximum number of messages per socket in a buffer.
	maxMessageBufferSize = 16
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

// NewSocket creates a new socket.
func NewSocket(s Session) *Socket {
	return &Socket{
		Session: s,
		msgs:    make(chan Event, maxMessageBufferSize),
	}
}

// Assigns returns the data currently assigned to this
// socket.
func (s *Socket) Assigns() interface{} {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	return s.data
}

// Assign set data to this socket. This will happen automatically
// if you return data from and `EventHander`.
func (s *Socket) Assign(data interface{}) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	s.data = data
}

// Send an event to this socket.
func (s *Socket) Send(msg Event) {
	select {
	case s.msgs <- msg:
	default:
		go s.closeSlow()
	}
}

// mount passes this socket to the handlers mount func. This returns data
// which we then set to the socket to store.
func (s *Socket) mount(ctx context.Context, h *Handler, r *http.Request, connected bool) error {
	data, err := h.Mount(ctx, h, r, s, connected)
	if err != nil {
		return fmt.Errorf("mount error: %w", err)
	}
	s.Assign(data)
	return nil
}

// render passes this socket to the handlers render func. This generates
// the HTML we should be showing to the socket. A diff is then run against
// previosuly generated HTML and patches sent to the socket.
func (s *Socket) render(ctx context.Context, h *Handler) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	// Render handler.
	output, err := h.Render(ctx, s.data)
	if err != nil {
		return fmt.Errorf("render error: %w", err)
	}
	node, err := html.Parse(output)
	if err != nil {
		return fmt.Errorf("html parse error: %w", err)
	}
	pruneTree(node)

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
			s.Send(msg)
		}
	}
	s.currentRender = node

	return nil
}

// assignWS connect a web socket to a socket.
func (s *Socket) assignWS(ws *websocket.Conn) {
	s.closeSlow = func() {
		ws.Close(websocket.StatusPolicyViolation, "socket too slow to keep up with messages")
	}
}
