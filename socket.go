package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"github.com/rs/xid"
	"golang.org/x/net/html"
)

const (
	// maxMessageBufferSize the maximum number of messages per socket in a buffer.
	maxMessageBufferSize = 16
)

var _ Socket = &BaseSocket{}

type SocketID string

// Socket describes a connected user, and the state that they
// are in.
type Socket interface {
	// ID return an ID for this socket.
	ID() SocketID
	// Assigns returns the data currently assigned to this
	// socket.
	Assigns() interface{}
	// Assign set data to this socket. This will happen automatically
	// if you return data from and `EventHander`.
	Assign(data interface{})
	// Connected returns true if this socket is connected via the websocket.
	Connected() bool
	// Self send an event to this socket itself. Will be handled in the
	// handlers HandleSelf function.
	Self(ctx context.Context, event string, data interface{}) error
	// Broadcast send an event to all sockets on this same handler.
	Broadcast(event string, data interface{}) error
	// Send an event to this socket's client, to be handled there.
	Send(event string, data interface{}, options ...EventConfig) error
	// PatchURL sends an event to the client to update the
	// query params in the URL.
	PatchURL(values url.Values)
	// Redirect sends a redirect event to the client. This will trigger the browser to
	// redirect to a URL.
	Redirect(u *url.URL)
	// LatestRender return the latest render that this socket generated.
	LatestRender() *html.Node
	// UpdateRender set the latest render.
	UpdateRender(render *html.Node)
	// Session returns the sockets session.
	Session() Session
	// Messages returns the channel of events on this socket.
	Messages() chan Event
}

// BaseSocket describes a socket from the outside.
type BaseSocket struct {
	session Session
	id      SocketID

	handler       Handler
	connected     bool
	currentRender *html.Node
	msgs          chan Event
	closeSlow     func()

	data   interface{}
	dataMu sync.Mutex
}

// NewBaseSocket creates a new default socket.
func NewBaseSocket(s Session, h Handler, connected bool) *BaseSocket {
	return &BaseSocket{
		session:   s,
		handler:   h,
		connected: connected,
		msgs:      make(chan Event, maxMessageBufferSize),
	}
}

// ID generate a unique ID for this socket.
func (s *BaseSocket) ID() SocketID {
	if s.id == "" {
		s.id = SocketID(xid.New().String())
	}
	return s.id
}

// Assigns returns the data currently assigned to this
// socket.
func (s *BaseSocket) Assigns() interface{} {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	return s.data
}

// Assign set data to this socket. This will happen automatically
// if you return data from and `EventHander`.
func (s *BaseSocket) Assign(data interface{}) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	s.data = data
}

// Connected returns if this socket is connected via the websocket.
func (s *BaseSocket) Connected() bool {
	return s.connected
}

// Self send an event to this socket itself. Will be handled in the
// handlers HandleSelf function.
func (s *BaseSocket) Self(ctx context.Context, event string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not encode data for self: %w", err)
	}
	msg := Event{T: event, Data: payload}
	s.handler.self(ctx, s, msg)
	return nil
}

// Broadcast send an event to all sockets on this same handler.
func (s *BaseSocket) Broadcast(event string, data interface{}) error {
	return s.handler.Broadcast(event, data)
}

// Send an event to this socket's client, to be handled there.
func (s *BaseSocket) Send(event string, data interface{}, options ...EventConfig) error {
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
func (s *BaseSocket) PatchURL(values url.Values) {
	s.Send(EventParams, values.Encode())
}

// Redirect sends a redirect event to the client. This will trigger the browser to
// redirect to a URL.
func (s *BaseSocket) Redirect(u *url.URL) {
	s.Send(EventRedirect, u.String())
}

func (s *BaseSocket) LatestRender() *html.Node {
	return s.currentRender
}
func (s *BaseSocket) UpdateRender(render *html.Node) {
	s.currentRender = render
}
func (s *BaseSocket) Session() Session {
	return s.session
}
func (s *BaseSocket) Messages() chan Event {
	return s.msgs
}
