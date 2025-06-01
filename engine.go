package live

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

// EngineConfig applies configuration to an engine.
type EngineConfig func(e *Engine) error

// WithWebsocketAcceptOptions apply websocket accept options to the HTTP engine.
func WithWebsocketAcceptOptions(options *websocket.AcceptOptions) EngineConfig {
	return func(e *Engine) error {
		e.acceptOptions = options
		return nil
	}
}

// WithSocketStateStore set the engines socket state store.
func WithSocketStateStore(sss SocketStateStore) EngineConfig {
	return func(e *Engine) error {
		e.socketStateStore = sss
		return nil
	}
}

func WithWebsocketMaxMessageSize(n int64) EngineConfig {
	return func(e *Engine) error {
		n = max(n, -1)
		e.MaxMessageSize = n
		return nil
	}
}

// BroadcastHandler a way for processes to communicate.
type BroadcastHandler func(ctx context.Context, e *Engine, msg Event)

// Engine handles live inner workings.
type Engine struct {
	// Handler implements all the developer defined logic.
	Handler *Handler

	// BroadcastLimiter limit broadcast ratehandler.
	BroadcastLimiter *rate.Limiter
	// broadcast handle a broadcast.
	BroadcastHandler BroadcastHandler

	// socket handling channels.
	addSocketC      chan engineAddSocket
	getSocketC      chan engineGetSocket
	deleteSocketC   chan engineDeleteSocket
	iterateSocketsC chan engineIterateSockets

	// IgnoreFaviconRequest setting to ignore requests for /favicon.ico.
	IgnoreFaviconRequest bool

	// MaxUploadSize the maximum upload size in bytes to allow. This defaults
	// too 100MB.
	MaxUploadSize int64

	// MaxMessageSize is the maximum size of websocket messages before they are rejected. Defaults to 32K (32768). Can be set to -1 to disable.
	MaxMessageSize int64

	// UploadStagingLocation where uploads are stored before they are consumed. This defaults
	// too the default OS temp directory.
	UploadStagingLocation string

	acceptOptions    *websocket.AcceptOptions
	socketStateStore SocketStateStore
}

type engineAddSocket struct {
	Socket *Socket
	resp   chan struct{}
}

type engineGetSocket struct {
	ID   SocketID
	resp chan *Socket
	err  chan error
}

type engineDeleteSocket struct {
	ID   SocketID
	resp chan struct{}
}

type engineIterateSockets struct {
	resp chan *Socket
	done chan bool
}

func (e *Engine) operate(ctx context.Context) {
	socketMap := map[SocketID]*Socket{}
	for {
		select {
		case op := <-e.addSocketC:
			socketMap[op.Socket.ID()] = op.Socket
			op.resp <- struct{}{}
		case op := <-e.getSocketC:
			s, ok := socketMap[op.ID]
			if !ok {
				op.err <- ErrNoSocket
				continue
			}
			op.resp <- s
		case op := <-e.deleteSocketC:
			delete(socketMap, op.ID)
			op.resp <- struct{}{}
		case op := <-e.iterateSocketsC:
			for _, s := range socketMap {
				op.resp <- s
			}
			op.done <- true
		case <-ctx.Done():
			return
		}
	}
}

// NewHttpHandler serve the handler.
func NewHttpHandler(ctx context.Context, h *Handler, configs ...EngineConfig) *Engine {
	const maxUploadSize = 100 * 1024 * 1024
	e := &Engine{
		BroadcastLimiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
		BroadcastHandler: func(ctx context.Context, h *Engine, msg Event) {
			h.self(ctx, nil, msg)
		},
		IgnoreFaviconRequest: true,
		MaxUploadSize:        maxUploadSize,
		MaxMessageSize:       32768,
		Handler:              h,
		addSocketC:           make(chan engineAddSocket),
		getSocketC:           make(chan engineGetSocket),
		deleteSocketC:        make(chan engineDeleteSocket),
		iterateSocketsC:      make(chan engineIterateSockets),
	}
	for _, conf := range configs {
		if err := conf(e); err != nil {
			slog.Warn(fmt.Sprintf("could not apply config to engine: %s", err))
		}
	}
	if e.socketStateStore == nil {
		e.socketStateStore = NewMemorySocketStateStore(ctx)
	}
	go e.operate(ctx)
	return e
}

// Broadcast send a message to all sockets connected to this engine.
func (e *Engine) Broadcast(event string, data any) error {
	ev := Event{T: event, SelfData: data}
	ctx := context.Background()
	e.BroadcastLimiter.Wait(ctx)
	e.BroadcastHandler(ctx, e, ev)
	return nil
}

// self sends a message to the socket on this engine.
func (e *Engine) self(ctx context.Context, sock *Socket, msg Event) {
	// If the socket is nil, this is broadcast message.
	if sock == nil {
		op := engineIterateSockets{
			resp: make(chan *Socket),
			done: make(chan bool),
		}
		e.iterateSocketsC <- op
		for {
			select {
			case socket := <-op.resp:
				e.handleEmittedEvent(ctx, socket, msg)
			case <-op.done:
				return
			}
		}
	} else {
		if err := e.hasSocket(sock); err != nil {
			return
		}
		e.handleEmittedEvent(ctx, sock, msg)
	}
}

func (e *Engine) handleEmittedEvent(ctx context.Context, s *Socket, msg Event) {
	if err := e.handleSelf(ctx, msg.T, s, msg); err != nil {
		slog.Error("server event error", "err", err)
	}
	render, err := RenderSocket(ctx, e, s)
	if err != nil {
		slog.Error("socket render error", "err", err)
	}
	s.UpdateRender(render)
}

// AddSocket add a socket to the engine.
func (e *Engine) AddSocket(sock *Socket) {
	op := engineAddSocket{
		Socket: sock,
		resp:   make(chan struct{}),
	}
	defer close(op.resp)
	e.addSocketC <- op
	<-op.resp
}

// GetSocket get a socket from a session.
func (e *Engine) GetSocket(ID SocketID) (*Socket, error) {
	op := engineGetSocket{
		ID:   ID,
		resp: make(chan *Socket),
		err:  make(chan error),
	}
	defer close(op.resp)
	defer close(op.err)
	e.getSocketC <- op
	select {
	case s := <-op.resp:
		return s, nil
	case err := <-op.err:
		return nil, err
	}
}

// DeleteSocket remove a socket from the engine.
func (e *Engine) DeleteSocket(sock *Socket) {
	op := engineDeleteSocket{
		ID:   sock.ID(),
		resp: make(chan struct{}),
	}
	defer close(op.resp)
	e.deleteSocketC <- op
	<-op.resp
	if err := e.Handler.UnmountHandler(sock); err != nil {
		slog.Error("socket unmount error", "err", err)
	}
	e.socketStateStore.Delete(sock.ID())
}

// CallEvent route an event to the correct handler.
func (e *Engine) CallEvent(ctx context.Context, t string, sock *Socket, msg Event) error {
	handler, err := e.Handler.getEvent(t)
	if err != nil {
		return err
	}

	params, err := msg.Params()
	if err != nil {
		return fmt.Errorf("received message and could not extract params: %w", err)
	}

	data, err := handler(ctx, sock, params)
	if err != nil {
		return err
	}
	sock.Assign(data)

	return nil
}

// handleSelf route an event to the correct handler.
func (e *Engine) handleSelf(ctx context.Context, t string, sock *Socket, msg Event) error {
	handler, err := e.Handler.getSelf(t)
	if err != nil {
		return fmt.Errorf("no self event handler for %s: %w", t, ErrNoEventHandler)
	}

	data, err := handler(ctx, sock, msg.SelfData)
	if err != nil {
		return fmt.Errorf("handler self event handler error [%s]: %w", t, err)
	}
	sock.Assign(data)

	return nil
}

// CallParams on params change run the handler.
func (e *Engine) CallParams(ctx context.Context, sock *Socket, msg Event) error {
	params, err := msg.Params()
	if err != nil {
		return fmt.Errorf("received params message and could not extract params: %w", err)
	}

	for _, ph := range e.Handler.paramsHandlers {
		data, err := ph(ctx, sock, params)
		if err != nil {
			return fmt.Errorf("handler params handler error: %w", err)
		}
		sock.Assign(data)
	}

	return nil
}

// hasSocket check a socket is there error if it isn't connected or
// doesn't exist.
func (e *Engine) hasSocket(s *Socket) error {
	_, err := e.GetSocket(s.ID())
	if err != nil {
		return ErrNoSocket
	}
	return nil
}

// ServeHTTP serves this handler.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		if e.IgnoreFaviconRequest {
			w.WriteHeader(404)
			return
		}
	}

	// Check if we are going to upgrade to a websocket.
	upgrade := slices.Contains(r.Header["Upgrade"], "websocket")

	ctx := httpContext(w, r)

	if !upgrade {
		switch r.Method {
		case http.MethodPost:
			e.post(ctx, w, r)
		default:
			e.get(ctx, w, r)
		}
		return
	}

	// Upgrade to the websocket version.
	e.serveWS(ctx, w, r)
}

// post handler.
func (e *Engine) post(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Get socket.
	sock, err := NewSocketFromRequest(ctx, e, r)
	if err != nil {
		e.Handler.ErrorHandler(ctx, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, e.MaxUploadSize)
	if err := r.ParseMultipartForm(e.MaxUploadSize); err != nil {
		e.Handler.ErrorHandler(ctx, fmt.Errorf("could not parse form for uploads: %w", err))
		return
	}

	uploadDir := filepath.Join(e.UploadStagingLocation, string(sock.ID()))
	if e.UploadStagingLocation == "" {
		uploadDir, err = os.MkdirTemp("", string(sock.ID()))
		if err != nil {
			e.Handler.ErrorHandler(ctx, fmt.Errorf("%s upload dir creation failed: %w", sock.ID(), err))
			return
		}
	}

	for _, config := range sock.UploadConfigs() {
		for _, fileHeader := range r.MultipartForm.File[config.Name] {
			u := uploadFromFileHeader(fileHeader)
			sock.AssignUpload(config.Name, u)
			handleFileUpload(e, sock, config, u, uploadDir, fileHeader)

			render, err := RenderSocket(ctx, e, sock)
			if err != nil {
				e.Handler.ErrorHandler(ctx, err)
				return
			}
			sock.UpdateRender(render)
		}
	}
}

func uploadFromFileHeader(fh *multipart.FileHeader) *Upload {
	return &Upload{
		Name: fh.Filename,
		Size: fh.Size,
	}
}

func handleFileUpload(h *Engine, sock *Socket, config *UploadConfig, u *Upload, uploadDir string, fileHeader *multipart.FileHeader) {
	// Check file claims to be within the max size.
	if fileHeader.Size > config.MaxSize {
		u.Errors = append(u.Errors, fmt.Errorf("%s greater than max allowed size of %d", fileHeader.Filename, config.MaxSize))
		return
	}

	// Open the incoming file.
	file, err := fileHeader.Open()
	if err != nil {
		u.Errors = append(u.Errors, fmt.Errorf("could not open %s for upload: %w", fileHeader.Filename, err))
		return
	}
	defer file.Close()

	// Check the actual filetype.
	buff := make([]byte, 512)
	_, err = file.Read(buff)
	if err != nil {
		u.Errors = append(u.Errors, fmt.Errorf("could not check %s for type: %w", fileHeader.Filename, err))
		return
	}
	filetype := http.DetectContentType(buff)
	allowed := slices.Contains(config.Accept, filetype)
	if !allowed {
		u.Errors = append(u.Errors, fmt.Errorf("%s filetype is not allowed", fileHeader.Filename))
		return
	}
	u.Type = filetype

	// Rewind to start of the
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		u.Errors = append(u.Errors, fmt.Errorf("%s rewind error: %w", fileHeader.Filename, err))
		return
	}

	f, err := os.Create(filepath.Join(uploadDir, fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(fileHeader.Filename))))
	if err != nil {
		u.Errors = append(u.Errors, fmt.Errorf("%s upload file creation failed: %w", fileHeader.Filename, err))
		return
	}
	defer f.Close()
	u.internalLocation = f.Name()
	u.Name = fileHeader.Filename

	written, err := io.Copy(f, io.TeeReader(file, &UploadProgress{Upload: u, Engine: h, Socket: sock}))
	if err != nil {
		u.Errors = append(u.Errors, fmt.Errorf("%s upload failed: %w", fileHeader.Filename, err))
		return
	}
	u.Size = written
}

// get renderer.
func (e *Engine) get(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Get socket.
	sock := NewSocket(ctx, e, "")

	// Write ID to cookie.
	sock.WriteFlashCookie(w)

	// Run mount, this generates the state for the page we are on.
	data, err := e.Handler.MountHandler(ctx, sock)
	if err != nil {
		e.Handler.ErrorHandler(ctx, err)
		return
	}
	sock.Assign(data)

	// Handle any query parameters that are on the page.
	for _, ph := range e.Handler.paramsHandlers {
		data, err := ph(ctx, sock, NewParamsFromRequest(r))
		if err != nil {
			e.Handler.ErrorHandler(ctx, err)
			return
		}
		sock.Assign(data)
	}

	// Render the HTML to display the page.
	render, err := RenderSocket(ctx, e, sock)
	if err != nil {
		e.Handler.ErrorHandler(ctx, err)
		return
	}
	sock.UpdateRender(render)

	var rendered bytes.Buffer
	html.Render(&rendered, render)

	w.WriteHeader(200)
	io.Copy(w, &rendered)
}

// serveWS serve a websocket request to the handler.
func (e *Engine) serveWS(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.UserAgent(), "Safari") {
		if e.acceptOptions == nil {
			e.acceptOptions = &websocket.AcceptOptions{}
		}
		e.acceptOptions.CompressionMode = websocket.CompressionDisabled
	}

	c, err := websocket.Accept(w, r, e.acceptOptions)
	if err != nil {
		e.Handler.ErrorHandler(ctx, err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "")
	c.SetReadLimit(e.MaxMessageSize)
	writeTimeout(ctx, time.Second*5, c, Event{T: EventConnect})
	{
		err := e._serveWS(ctx, r, c)
		if errors.Is(err, context.Canceled) {
			return
		}
		switch websocket.CloseStatus(err) {
		case websocket.StatusNormalClosure:
			return
		case websocket.StatusGoingAway:
			return
		case -1:
			return
		default:
			slog.Error("ws closed", "err", fmt.Errorf("ws closed with status (%d): %w", websocket.CloseStatus(err), err))
			return
		}
	}
}

// _serveWS implement the logic for a web socket connection.
func (e *Engine) _serveWS(ctx context.Context, r *http.Request, c *websocket.Conn) error {
	// Get the sessions socket and register it with the server.
	sock, err := NewSocketFromRequest(ctx, e, r)
	if err != nil {
		return fmt.Errorf("failed precondition: %w", err)
	}
	sock.assignWS(c)
	e.AddSocket(sock)
	defer e.DeleteSocket(sock)

	// Internal errors.
	internalErrors := make(chan error)

	// Event errors.
	eventErrors := make(chan ErrorEvent)

	// Handle events coming from the websocket connection.
	go func() {
		for {
			t, d, err := c.Read(ctx)
			if err != nil {
				internalErrors <- err
				break
			}
			switch t {
			case websocket.MessageText:
				var m Event
				if err := json.Unmarshal(d, &m); err != nil {
					internalErrors <- err
					break
				}
				switch m.T {
				case EventParams:
					if err := e.CallParams(ctx, sock, m); err != nil {
						switch {
						case errors.Is(err, ErrNoEventHandler):
							slog.Error("event params error", "event", m, "err", err)
						default:
							eventErrors <- ErrorEvent{Source: m, Err: err.Error()}
						}
					}
				default:
					if err := e.CallEvent(ctx, m.T, sock, m); err != nil {
						switch {
						case errors.Is(err, ErrNoEventHandler):
							slog.Error("event default error", "event", m, "err", err)
						default:
							eventErrors <- ErrorEvent{Source: m, Err: err.Error()}
						}
					}
				}
				render, err := RenderSocket(ctx, e, sock)
				if err != nil {
					internalErrors <- fmt.Errorf("socket handle error: %w", err)
				} else {
					sock.UpdateRender(render)
				}
				if err := sock.Send(EventAck, nil, WithID(m.ID)); err != nil {
					internalErrors <- fmt.Errorf("socket send error: %w", err)
				}
			case websocket.MessageBinary:
				slog.Warn("binary messages unhandled")
			}
		}
		close(internalErrors)
		close(eventErrors)
	}()

	// Run mount again now that eh socket is connected, passing true indicating
	// a connection has been made.
	data, err := e.Handler.MountHandler(ctx, sock)
	if err != nil {
		return fmt.Errorf("socket mount error: %w", err)
	}
	sock.Assign(data)

	// Run params again now that the socket is connected.
	for _, ph := range e.Handler.paramsHandlers {
		data, err := ph(ctx, sock, NewParamsFromRequest(r))
		if err != nil {
			return fmt.Errorf("socket params error: %w", err)
		}
		sock.Assign(data)
	}

	// Run render now that we are connected for the first time and we have just
	// mounted again. This will generate and send any patches if there have
	// been changes.
	render, err := RenderSocket(ctx, e, sock)
	if err != nil {
		return fmt.Errorf("socket render error: %w", err)
	}
	sock.UpdateRender(render)

	// Send events to the websocket connection.
	for {
		select {
		case msg := <-sock.msgs:
			if err := writeTimeout(ctx, time.Second*5, c, msg); err != nil {
				return fmt.Errorf("writing to socket error: %w", err)
			}
		case ee := <-eventErrors:
			d, err := json.Marshal(ee)
			if err != nil {
				return fmt.Errorf("writing to socket error: %w", err)
			}
			if err := writeTimeout(ctx, time.Second*5, c, Event{T: EventError, Data: d}); err != nil {
				return fmt.Errorf("writing to socket error: %w", err)
			}
		case err := <-internalErrors:
			if err != nil {
				d, err := json.Marshal(err.Error())
				if err != nil {
					return fmt.Errorf("writing to socket error: %w", err)
				}
				if err := writeTimeout(ctx, time.Second*5, c, Event{T: EventError, Data: d}); err != nil {
					return fmt.Errorf("writing to socket error: %w", err)
				}
				// Something catastrophic has happened.
				return fmt.Errorf("internal error: %w", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}
