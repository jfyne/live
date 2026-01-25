package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/rs/xid"
	"golang.org/x/net/html"
)

const (
	// maxMessageBufferSize the maximum number of messages per socket in a buffer.
	maxMessageBufferSize = 16

	// cookieSocketID name for a cookie which holds the current socket ID.
	cookieSocketID = "_psid"

	// infiniteTTL
	infiniteTTL = 10_000 * (24 * time.Hour)
)

type SocketID string

// Socket describes a socket from the outside.
type Socket struct {
	id SocketID

	engine        *Engine
	connected     bool
	currentRender *html.Node
	renderMu      sync.Mutex
	msgs          chan Event
	closeSlow     func()

	uploadMu      sync.Mutex
	uploadConfigs []*UploadConfig
	uploads       UploadContext

	selfChan chan socketSelfOp
}

// Render executes the template, computes a diff against the previous render,
// sends any patches to the client, and updates the stored render tree.
// It is safe for concurrent use.
func (s *Socket) Render(ctx context.Context) error {
	s.renderMu.Lock()
	defer s.renderMu.Unlock()
	render, err := renderSocket(ctx, s.engine, s)
	if err != nil {
		return err
	}
	s.updateRender(render)
	return nil
}

type socketSelfOp struct {
	Event Event
	resp  chan bool
	err   chan error
}

// NewID returns a new ID.
func NewID() string {
	return xid.New().String()
}

// NewSocketFromRequest creates a new default socket from a request.
func NewSocketFromRequest(ctx context.Context, e *Engine, r *http.Request) (*Socket, error) {
	sockID, err := socketIDFromReq(r)
	if err != nil {
		return nil, fmt.Errorf("socket id not found: %w", err)
	}

	existingSock, err := e.GetSocket(sockID)
	if err == nil {
		return existingSock, nil
	}

	return NewSocket(ctx, e, sockID), nil
}

func socketIDFromReq(r *http.Request) (SocketID, error) {
	c, err := r.Cookie(cookieSocketID)
	if err == nil {
		return SocketID(c.Value), nil
	}

	v := r.FormValue(cookieSocketID)
	if v != "" {
		return SocketID(v), nil
	}

	return "", fmt.Errorf("socket id not found in cookie or form data")
}

// NewSocket creates a new default socket.
func NewSocket(ctx context.Context, e *Engine, withID SocketID) *Socket {
	s := &Socket{
		id:            withID,
		engine:        e,
		connected:     withID != "",
		closeSlow:     func() {},
		uploadConfigs: []*UploadConfig{},
		msgs:          make(chan Event, maxMessageBufferSize),
		selfChan:      make(chan socketSelfOp),
	}
	if withID == "" {
		s.id = SocketID(NewID())
	}
	go s.operate(ctx)
	return s
}

func (s *Socket) WriteFlashCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieSocketID,
		Value:    string(s.id),
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   1,
	})
}

// ID gets the socket ID.
func (s *Socket) ID() SocketID {
	return s.id
}

// Assigns returns the data currently assigned to this
// socket.
func (s *Socket) Assigns() any {
	state, _ := s.engine.socketStateStore.Get(s.id)
	return state.Data
}

// Assign sets data to this socket. This will happen automatically
// if you return data from an `EventHander`.
func (s *Socket) Assign(data any) {
	state, _ := s.engine.socketStateStore.Get(s.id)
	state.Data = data
	ttl := 10 * time.Second
	if s.connected {
		ttl = infiniteTTL
	}
	s.engine.socketStateStore.Set(s.id, state, ttl)
}

// Connected returns if this socket is connected via the websocket.
func (s *Socket) Connected() bool {
	return s.connected
}

// Self sends an event to this socket itself. Will be handled in the
// handlers HandleSelf function.
func (s *Socket) Self(ctx context.Context, event string, data any) error {
	op := socketSelfOp{
		Event: Event{T: event, SelfData: data},
		resp:  make(chan bool),
		err:   make(chan error),
	}
	s.selfChan <- op
	select {
	case <-op.resp:
		return nil
	case err := <-op.err:
		return err
	}
}

func (s *Socket) operate(ctx context.Context) {
	for {
		select {
		case op := <-s.selfChan:
			s.engine.self(ctx, s, op.Event)
			op.resp <- true
		case <-ctx.Done():
			return
		}
	}
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
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	s.uploadConfigs = append(s.uploadConfigs, config)
}

// UploadConfigs returns the configs for this socket.
func (s *Socket) UploadConfigs() []*UploadConfig {
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	out := make([]*UploadConfig, len(s.uploadConfigs))
	copy(out, s.uploadConfigs)
	return out
}

// Uploads returns the sockets uploads.
func (s *Socket) Uploads() UploadContext {
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	out := make(UploadContext, len(s.uploads))
	for k, v := range s.uploads {
		cp := make([]*Upload, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// AssignUpload sets uploads to this socket.
func (s *Socket) AssignUpload(config string, upload *Upload) {
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
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
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	s.uploads = map[string][]*Upload{}
}

// ClearUpload clears a specific upload from this socket.
func (s *Socket) ClearUpload(config string, upload *Upload) {
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	if s.uploads == nil {
		return
	}
	if _, ok := s.uploads[config]; !ok {
		return
	}
	for idx, u := range s.uploads[config] {
		if u.Name == upload.Name {
			s.uploads[config] = slices.Delete(s.uploads[config], idx, idx+1)
			return
		}
	}
}

// latestRender returns the last render result of this socket.
func (s *Socket) latestRender() *html.Node {
	return s.currentRender
}

// updateRender replaces the last render result of this socket.
func (s *Socket) updateRender(render *html.Node) {
	s.currentRender = render
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
