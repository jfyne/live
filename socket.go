package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"golang.org/x/net/html"
	"nhooyr.io/websocket"
)

const (
	// maxMessageBufferSize the maximum number of messages per socket in a buffer.
	maxMessageBufferSize = 16
)

//var _ Socket = &Socket{}

type SocketID string

//// Socket describes a connected user, and the state that they
//// are in.
//type Socket interface {
//	// ID return an ID for this socket.
//	ID() SocketID
//	// Assigns returns the data currently assigned to this
//	// socket.
//	Assigns() any
//	// Assign set data to this socket. This will happen automatically
//	// if you return data from an `EventHander`.
//	Assign(data any)
//	// Connected returns true if this socket is connected via the websocket.
//	Connected() bool
//	// Self send an event to this socket itself. Will be handled in the
//	// handlers HandleSelf function.
//	Self(ctx context.Context, event string, data any) error
//	// Broadcast send an event to all sockets on this same engine.
//	Broadcast(event string, data any) error
//	// Send an event to this socket's client, to be handled there.
//	Send(event string, data any, options ...EventConfig) error
//	// PatchURL sends an event to the client to update the
//	// query params in the URL.
//	PatchURL(values url.Values)
//	// Redirect sends a redirect event to the client. This will trigger the browser to
//	// redirect to a URL.
//	Redirect(u *url.URL)
//	// AllowUploads indicates that his socket should allow uploads.
//	AllowUploads(config *UploadConfig)
//	// UploadConfigs return the list of configures uploads for this socket.
//	UploadConfigs() []*UploadConfig
//	// Uploads returns uploads to this socket.
//	Uploads() UploadContext
//	// AssignUploads set uploads to a upload config on this socket.
//	AssignUpload(config string, upload *Upload)
//	// ClearUploads clears the sockets upload map.
//	ClearUploads()
//	// ClearUpload clear a specific upload.
//	ClearUpload(config string, upload *Upload)
//	// LatestRender return the latest render that this socket generated.
//	LatestRender() *html.Node
//	// UpdateRender set the latest render.
//	UpdateRender(render *html.Node)
//	// Session returns the sockets session.
//	Session() Session
//	// Messages returns the channel of events on this socket.
//	Messages() chan Event
//}

// Socket describes a socket from the outside.
type Socket struct {
	session Session
	id      SocketID

	engine        *Engine
	connected     bool
	currentRender *html.Node
	msgs          chan Event
	closeSlow     func()

	uploadConfigs []*UploadConfig
	uploads       UploadContext

	data   any
	dataMu sync.Mutex
	selfMu sync.Mutex
}

// NewSocket creates a new default socket.
func NewSocket(s Session, e *Engine, connected bool) *Socket {
	return &Socket{
		session:       s,
		engine:        e,
		connected:     connected,
		uploadConfigs: []*UploadConfig{},
		msgs:          make(chan Event, maxMessageBufferSize),
	}
}

// ID generates a unique ID for this socket.
func (s *Socket) ID() SocketID {
	if s.id == "" {
		s.id = SocketID(NewID())
	}
	return s.id
}

// Assigns returns the data currently assigned to this
// socket.
func (s *Socket) Assigns() any {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	return s.data
}

// Assign sets data to this socket. This will happen automatically
// if you return data from an `EventHander`.
func (s *Socket) Assign(data any) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	s.data = data
}

// Connected returns if this socket is connected via the websocket.
func (s *Socket) Connected() bool {
	return s.connected
}

// Self sends an event to this socket itself. Will be handled in the
// handlers HandleSelf function.
func (s *Socket) Self(ctx context.Context, event string, data any) error {
	s.selfMu.Lock()
	defer s.selfMu.Unlock()
	msg := Event{T: event, SelfData: data}
	s.engine.self(ctx, s, msg)
	return nil
}

// Broadcast sends an event to all sockets on this same engine.
func (s *Socket) Broadcast(event string, data any) error {
	return s.engine.Broadcast(event, data)
}

// Send an event to this socket's client, to be handled there.
func (s *Socket) Send(event string, data any, options ...EventConfig) error {
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

// AllowUploads indicates that his socket should accept uploads.
func (s *Socket) AllowUploads(config *UploadConfig) {
	s.uploadConfigs = append(s.uploadConfigs, config)
}

// UploadConfigs returns the configs for this socket.
func (s *Socket) UploadConfigs() []*UploadConfig {
	return s.uploadConfigs
}

// Uploads returns the sockets uploads.
func (s *Socket) Uploads() UploadContext {
	return s.uploads
}

// AssignUpload sets uploads to this socket.
func (s *Socket) AssignUpload(config string, upload *Upload) {
	if s.uploads == nil {
		s.uploads = map[string][]*Upload{}
	}
	if _, ok := s.uploads[config]; !ok {
		s.uploads[config] = []*Upload{}
	}
	for idx, u := range s.uploads[config] {
		if u.Name == upload.Name {
			s.uploads[config][idx] = upload
			return
		}
	}
	s.uploads[config] = append(s.uploads[config], upload)
}

// ClearUploads clears this sockets upload map.
func (s *Socket) ClearUploads() {
	s.uploads = map[string][]*Upload{}
}

// ClearUpload clears a specific upload from this socket.
func (s *Socket) ClearUpload(config string, upload *Upload) {
	if s.uploads == nil {
		s.uploads = map[string][]*Upload{}
	}
	if _, ok := s.uploads[config]; !ok {
		return
	}
	for idx, u := range s.uploads[config] {
		if u.Name == upload.Name {
			s.uploads[config] = append(s.uploads[config][:idx], s.uploads[config][idx+1:]...)
			return
		}
	}
}

// LastRender returns the last render result of this socket.
func (s *Socket) LatestRender() *html.Node {
	return s.currentRender
}

// UpdateRender replaces the last render result of this socket.
func (s *Socket) UpdateRender(render *html.Node) {
	s.currentRender = render
}

// Session returns the session of this socket.
func (s *Socket) Session() Session {
	return s.session
}

// Messages returns a channel of event messages sent and received by this socket.
func (s *Socket) Messages() chan Event {
	return s.msgs
}

// assignWS connect a web socket to a socket.
func (s *Socket) assignWS(ws *websocket.Conn) {
	s.closeSlow = func() {
		ws.Close(websocket.StatusPolicyViolation, "socket too slow to keep up with messages")
	}
}
