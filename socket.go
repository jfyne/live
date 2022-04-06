package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

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
	// Broadcast send an event to all sockets on this same engine.
	Broadcast(event string, data interface{}) error
	// Send an event to this socket's client, to be handled there.
	Send(event string, data interface{}, options ...EventConfig) error
	// PatchURL sends an event to the client to update the
	// query params in the URL.
	PatchURL(values url.Values)
	// Redirect sends a redirect event to the client. This will trigger the browser to
	// redirect to a URL.
	Redirect(u *url.URL)
	// AllowUploads indicates that his socket should allow uploads.
	AllowUploads(config *UploadConfig)
	// UploadConfigs return the list of configures uploads for this socket.
	UploadConfigs() []*UploadConfig
	// Uploads returns uploads to this socket.
	Uploads() UploadContext
	// AssignUploads set uploads to a upload config on this socket.
	AssignUpload(config string, upload *Upload)
	// ClearUploads clears the sockets upload map.
	ClearUploads()
	// ClearUpload clear a specific upload.
	ClearUpload(config string, upload *Upload)
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

	engine        Engine
	connected     bool
	currentRender *html.Node
	msgs          chan Event
	closeSlow     func()

	uploadConfigs []*UploadConfig
	uploads       UploadContext

	data   interface{}
	dataMu sync.Mutex
}

// NewBaseSocket creates a new default socket.
func NewBaseSocket(s Session, e Engine, connected bool) *BaseSocket {
	return &BaseSocket{
		session:       s,
		engine:        e,
		connected:     connected,
		uploadConfigs: []*UploadConfig{},
		msgs:          make(chan Event, maxMessageBufferSize),
	}
}

// ID generate a unique ID for this socket.
func (s *BaseSocket) ID() SocketID {
	if s.id == "" {
		s.id = SocketID(NewID())
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
	msg := Event{T: event, SelfData: data}
	s.engine.self(ctx, s, msg)
	return nil
}

// Broadcast send an event to all sockets on this same engine.
func (s *BaseSocket) Broadcast(event string, data interface{}) error {
	return s.engine.Broadcast(event, data)
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

// AllowUploads indicates that his socket should accept uploads.
func (s *BaseSocket) AllowUploads(config *UploadConfig) {
	s.uploadConfigs = append(s.uploadConfigs, config)
}

// UploadConfigs return the configs for this socket.
func (s *BaseSocket) UploadConfigs() []*UploadConfig {
	return s.uploadConfigs
}

// Uploads return the sockets uploads.
func (s *BaseSocket) Uploads() UploadContext {
	return s.uploads
}

// AssignUpload set uploads to this socket.
func (s *BaseSocket) AssignUpload(config string, upload *Upload) {
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

// ClearUploads clear this sockets upload map.
func (s *BaseSocket) ClearUploads() {
	s.uploads = map[string][]*Upload{}
}

// ClearUpload clear a specific upload from this socket.
func (s *BaseSocket) ClearUpload(config string, upload *Upload) {
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
