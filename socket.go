package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	// The session for this socket.
	Session Session

	handler       *Handler
	connected     bool
	currentRender *html.Node
	msgs          chan Event
	closeSlow     func()

	data   interface{}
	dataMu sync.Mutex
}

// NewSocket creates a new socket.
func NewSocket(s Session, h *Handler, connected bool) *Socket {
	return &Socket{
		Session:   s,
		handler:   h,
		connected: connected,
		msgs:      make(chan Event, maxMessageBufferSize),
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

// Connected returns if this socket is connected via the websocket.
func (s *Socket) Connected() bool {
	return s.connected
}

// Self send an event to this socket itself. Will be handled in the
// handlers HandleSelf function.
func (s *Socket) Self(ctx context.Context, event string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not encode data for self: %w", err)
	}
	msg := Event{T: event, Data: payload}
	s.handler.self(ctx, s, msg)
	return nil
}

// Broadcast send an event to all sockets on this same handler.
func (s *Socket) Broadcast(event string, data interface{}) error {
	return s.handler.Broadcast(event, data)
}

// Send an event to this socket's client, to be handled there.
func (s *Socket) Send(event string, data interface{}, options ...EventConfig) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not encode data for send: %w", err)
	}
	msg := Event{T: event, Data: payload}
	for _, o := range options {
		if err := o(&msg); err != nil {
			return fmt.Errorf("could not configure event: %w", err)
		}
	}
	select {
	case s.msgs <- msg:
	default:
		go s.closeSlow()
	}
	return nil
}

// PatchURL sends an event to the client to update the
// query params in the URL.
func (s *Socket) PatchURL(values url.Values) {
	s.Send(EventParams, values.Encode())
}

// Redirect sends a redirect event to the client. This will trigger the browser to
// redirect to a URL.
func (s *Socket) Redirect(u *url.URL) {
	s.Send(EventRedirect, u.String())
}

// mount passes this socket to the handlers mount func. This returns data
// which we then set to the socket to store.
func (s *Socket) mount(ctx context.Context, h *Handler, r *http.Request) error {
	data, err := h.Mount(ctx, r, s)
	if err != nil {
		return fmt.Errorf("mount error: %w", err)
	}
	s.Assign(data)
	return nil
}

// params passes this socket to the handlers params func. This returns data
// which we then set to the socket to store.
func (s *Socket) params(ctx context.Context, h *Handler, r *http.Request) error {
	for _, ph := range h.paramsHandlers {
		data, err := ph(ctx, s, NewParamsFromRequest(r))
		if err != nil {
			return fmt.Errorf("params error: %w", err)
		}
		s.Assign(data)
	}
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
	shapeTree(node)

	// Get diff
	if s.currentRender != nil {
		patches, err := Diff(s.currentRender, node)
		if err != nil {
			return fmt.Errorf("diff error: %w", err)
		}
		if len(patches) != 0 {
			s.Send(EventPatch, patches)
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
