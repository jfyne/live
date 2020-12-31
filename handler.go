package live

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/sessions"
	"golang.org/x/net/html"
	"golang.org/x/time/rate"
	"nhooyr.io/websocket"
)

// MountHandler when mount is reached.
type MountHandler func(ctx context.Context, h *Handler, r *http.Request, c *Socket, connected bool) (interface{}, error)

// RenderHandler when the view is asked to render.
type RenderHandler func(ctx context.Context, t *template.Template, data interface{}) (io.Reader, error)

// HandlerConfig applies config to a handler.
type HandlerConfig func(h *Handler) error

// HandlerEvent an event sent by the handler.
type HandlerEvent struct {
	S   *Socket
	Msg Event
}

// Handler to be served by an HTTP server.
type Handler struct {
	// session store
	store      sessions.Store
	sessionKey string

	// Template for this view.
	t *template.Template

	// emitter is a channel to send messages back to the socket.
	emitter chan HandlerEvent

	// broadcastLimiter controls the rate limit applied to broadcasting
	// messages endpoint.
	// Defaults to one publish every 100ms with a burst of 8.
	broadcastLimiter *rate.Limiter

	// eventHandlers the map of event handlers.
	eventHandlers map[string]EventHandler

	// selfHandlers handle messages send to this view by server-side
	// entities.
	selfHandlers map[string]EventHandler

	Mount  MountHandler
	Render RenderHandler

	// All of our current sockets.
	socketsMu sync.Mutex
	socketMap map[*Socket]struct{}

	eventMu sync.Mutex
}

// NewHandler creates a new live handler.
func NewHandler(t *template.Template, sessionKey string, store sessions.Store, configs ...HandlerConfig) (*Handler, error) {
	h := &Handler{
		t:                t,
		store:            store,
		sessionKey:       sessionKey,
		emitter:          make(chan HandlerEvent),
		broadcastLimiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
		eventHandlers:    make(map[string]EventHandler),
		selfHandlers:     make(map[string]EventHandler),
		Mount: func(ctx context.Context, hd *Handler, r *http.Request, c *Socket, connected bool) (interface{}, error) {
			return nil, nil
		},
		Render: func(ctx context.Context, t *template.Template, data interface{}) (io.Reader, error) {
			var buf bytes.Buffer
			if t == nil {
				return nil, fmt.Errorf("default renderer: no template defined")
			}
			if err := t.ExecuteTemplate(&buf, "root.html", data); err != nil {
				return nil, err
			}
			return &buf, nil
		},
		socketMap: make(map[*Socket]struct{}),
	}

	for _, conf := range configs {
		if err := conf(h); err != nil {
			return nil, fmt.Errorf("could not apply config: %w", err)
		}
	}

	go StartHandler(h)
	return h, nil
}

// StartHandler run a handler so that it' events can be dealt with.
func StartHandler(h *Handler) {
	for {
		select {
		case m := <-h.emitter:
			go handleEmmittedEvent(h, m)
		}
	}
}

// ServeHTTP serves this handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if we are going to upgrade to a webscoket.
	upgrade := false
	for _, header := range r.Header["Upgrade"] {
		if header == "websocket" {
			upgrade = true
			break
		}
	}

	if !upgrade {
		// Serve the http version of the view.
		h.serveHTTP(w, r)
		return
	}

	// Upgrade to the webscoket version.
	h.serveWS(w, r)
	return
}

// serveHTTP serve an http request to the view.
func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	// Get session.
	session, err := h.getSession(r)
	if err != nil {
		log.Println("session get err", err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	// Get socket.
	sock := NewSocket(session)

	if err := sock.mount(r.Context(), h, r, false); err != nil {
		log.Println("socket mount err", err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	if err := sock.handleHandler(r.Context(), h); err != nil {
		log.Println("socket handle view err", err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	var rendered bytes.Buffer
	html.Render(&rendered, sock.currentRender)

	if err := h.saveSession(w, r, session); err != nil {
		log.Println("session save err", err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(200)
	io.Copy(w, &rendered)
}

// serveWS serve a websocket request to the view.
func (h *Handler) serveWS(w http.ResponseWriter, r *http.Request) {
	// Get the session from the http request.
	session, err := h.getSession(r)
	if err != nil {
		log.Println("get session for ws err", err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Println("websocket accept error", err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "")
	{
		err := h.socket(r.Context(), r, session, c)
		if errors.Is(err, context.Canceled) {
			return
		}
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
			websocket.CloseStatus(err) == websocket.StatusGoingAway {
			return
		}
		if err != nil {
			log.Println("websocket failure", err)
			return
		}
	}
}

// socket implement the handler for a socket.
func (h *Handler) socket(ctx context.Context, r *http.Request, session Session, c *websocket.Conn) error {
	// Get the sessions socket and register it with the server.
	sock := NewSocket(session)
	sock.assignWS(c)
	h.addSocket(sock)
	defer h.deleteSocket(sock)

	if err := sock.mount(ctx, h, r, true); err != nil {
		return fmt.Errorf("socket mount error: %w", err)
	}

	if err := sock.handleHandler(ctx, h); err != nil {
		return fmt.Errorf("socket handle error: %w", err)
	}

	// Handle events coming from the websocket connection.
	readError := make(chan error)
	go func() {
		for {
			t, d, err := c.Read(ctx)
			if err != nil {
				readError <- err
				break
			}
			switch t {
			case websocket.MessageText:
				var m Event
				if err := json.Unmarshal(d, &m); err != nil {
					readError <- err
					break
				}
				if err := h.handleEvent(m.T, sock, m); err != nil {
					if !errors.Is(err, ErrNoEventHandler) {
						readError <- err
						break
					} else {
						log.Println("event error", m, err)
					}
				}
				if err := sock.handleHandler(ctx, h); err != nil {
					readError <- fmt.Errorf("socket handle error: %w", err)
				}
			case websocket.MessageBinary:
				log.Println("binary messages unhandled")
			}
		}
		close(readError)
	}()

	// Send events to the websocket connection.
	for {
		select {
		case err := <-readError:
			if err != nil {
				writeTimeout(ctx, time.Second*5, c, Event{T: EventError, Data: err.Error()})
				return fmt.Errorf("read error: %w", err)
			}
		case msg := <-sock.msgs:
			if err := writeTimeout(ctx, time.Second*5, c, msg); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// addSocket add a socket to the handler.
func (h *Handler) addSocket(sock *Socket) {
	h.socketsMu.Lock()
	defer h.socketsMu.Unlock()
	h.socketMap[sock] = struct{}{}
}

// deleteSocket remove a socket from the handler.
func (h *Handler) deleteSocket(sock *Socket) {
	h.socketsMu.Lock()
	defer h.socketsMu.Unlock()
	delete(h.socketMap, sock)
}

// Self sends a message to the socket on this view.
func (h *Handler) Self(sock *Socket, msg Event) {
	h.emitter <- HandlerEvent{
		S:   sock,
		Msg: msg,
	}
}

// Broadcast send a message to all sockets connected to this view.
func (h *Handler) Broadcast(msg Event) {
	ctx := context.Background()
	h.broadcastLimiter.Wait(ctx)

	h.emitter <- HandlerEvent{
		Msg: msg,
	}
}

// HandleEvent handles an event that comes from the client. For example a click
// from `live-click="myevent"`.
func (h *Handler) HandleEvent(t string, handler EventHandler) {
	h.eventHandlers[t] = handler
}

// HandleSelf handles an event that comes from the view. For example calling
// view.Self(socket, msg) will be handled here.
func (h *Handler) HandleSelf(t string, handler EventHandler) {
	h.selfHandlers[t] = handler
}

// handleEvent route an event to the correct handler.
func (h *Handler) handleEvent(t string, sock *Socket, msg Event) error {
	handler, ok := h.eventHandlers[t]
	if !ok {
		return fmt.Errorf("no event handler for %s: %w", t, ErrNoEventHandler)
	}

	params, err := msg.Params()
	if err != nil {
		return fmt.Errorf("received message and could not extract params: %w", err)
	}

	data, err := handler(sock, params)
	if err != nil {
		return fmt.Errorf("view event handler error [%s]: %w", t, err)
	}
	sock.Assign(data)

	return nil
}

// handleSelf route an event to the correct handler.
func (h *Handler) handleSelf(t string, sock *Socket, msg Event) error {
	h.eventMu.Lock()
	defer h.eventMu.Unlock()

	handler, ok := h.selfHandlers[t]
	if !ok {
		return fmt.Errorf("no self event handler for %s: %w", t, ErrNoEventHandler)
	}

	params, err := msg.Params()
	if err != nil {
		return fmt.Errorf("received self message and could not extract params: %w", err)
	}

	data, err := handler(sock, params)
	if err != nil {
		return fmt.Errorf("view self event handler error [%s]: %w", t, err)
	}
	sock.Assign(data)

	return nil
}

// sockets returns all sockets connected to the handler.
func (h *Handler) sockets() []*Socket {
	h.socketsMu.Lock()
	defer h.socketsMu.Unlock()

	sockets := make([]*Socket, len(h.socketMap))
	idx := 0
	for socket := range h.socketMap {
		sockets[idx] = socket
		idx++
	}
	return sockets
}

// getSocket return a socket, error if it isn't connected or
// doensn't exist.
func (h *Handler) getSocket(s *Socket) (*Socket, error) {
	h.socketsMu.Lock()
	defer h.socketsMu.Unlock()
	_, ok := h.socketMap[s]
	if !ok {
		return nil, ErrNoSocket
	}
	return s, nil
}

func handleEmmittedEvent(h *Handler, he HandlerEvent) {
	// If the socket is nil, this is broadcast message.
	if he.S == nil {
		sockets := h.sockets()
		for _, socket := range sockets {
			_handleEmittedEvent(h, he, socket)
		}
	} else {
		if _, err := h.getSocket(he.S); err != nil {
			return
		}
		_handleEmittedEvent(h, he, he.S)
	}
}

func _handleEmittedEvent(h *Handler, he HandlerEvent, socket *Socket) {
	if err := h.handleSelf(he.Msg.T, socket, he.Msg); err != nil {
		log.Println("server event error", err)
	}
	if err := socket.handleHandler(context.Background(), h); err != nil {
		log.Println("socket handleView error", err)
	}
}

func (h *Handler) getSession(r *http.Request) (Session, error) {
	var sess Session
	session, err := h.store.Get(r, h.sessionKey)
	if err != nil {
		return NewSession(), err
	}

	vals, ok := session.Values[SessionKey]
	if !ok {
		// Create new connection.
		ns := NewSession()
		sess = ns
	}
	sess, ok = vals.(Session)
	if !ok {
		// Create new connection and set.
		ns := NewSession()
		sess = ns
	}
	return sess, nil
}

func (h *Handler) saveSession(w http.ResponseWriter, r *http.Request, session Session) error {
	c, err := h.store.Get(r, h.sessionKey)
	if err != nil {
		return err
	}
	c.Values[SessionKey] = session
	return c.Save(r, w)
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg Event) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(&msg)
	if err != nil {
		return fmt.Errorf("failed writeTimeout: %w", err)
	}

	return c.Write(ctx, websocket.MessageText, data)
}

// WithRootTemplate set the renderer to use a different root template. This changes the handlers
// Render function.
func WithRootTemplate(rootTemplate string) HandlerConfig {
	return func(h *Handler) error {
		h.Render = func(ctx context.Context, t *template.Template, data interface{}) (io.Reader, error) {
			var buf bytes.Buffer
			if err := t.ExecuteTemplate(&buf, rootTemplate, data); err != nil {
				return nil, err
			}
			return &buf, nil
		}
		return nil
	}
}
