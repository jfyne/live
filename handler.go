package live

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/time/rate"
	"nhooyr.io/websocket"
)

// MountHandler the func that is called by a handler to gather data to
// be rendered in a template. This is called on first GET and then later when
// the web socket first connects.
type MountHandler func(ctx context.Context, h *Handler, r *http.Request, c *Socket, connected bool) (interface{}, error)

// RenderHandler the func that is called to render the current state of the
// data for the socket.
type RenderHandler func(ctx context.Context, data interface{}) (io.Reader, error)

// ErrorHandler if an error occurs during the mount and render cycle
// a handler of this type will be called.
type ErrorHandler func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)

// HandlerConfig applies config to a handler.
type HandlerConfig func(h *Handler) error

// HandlerEvent an event sent by the handler.
type HandlerEvent struct {
	S   *Socket
	Msg Event
}

// Handler to be served by an HTTP server.
type Handler struct {
	// Mount a user should provide the mount function. This is what
	// is called on initial GET request and later when the websocket connects.
	// Data to render the view should be fetched here and returned.
	Mount MountHandler
	// Render is called to generate the HTML of a Socket. It is defined
	// by default and will render any template provided.
	Render RenderHandler
	// Error is called when an error occurs during the mount and render
	// stages of the handler lifecycle.
	Error ErrorHandler

	// session store
	sessionStore SessionStore

	// emitter is a channel to send messages back to the socket.
	emitter chan HandlerEvent

	// broadcastLimiter limit broadcast rate.
	broadcastLimiter *rate.Limiter

	// eventHandlers the map of client event handlers.
	eventHandlers map[string]EventHandler

	// selfHandlers the map of handler event handlers.
	selfHandlers map[string]EventHandler

	// paramsHandler the handle a change in URL parameters.
	paramsHandler EventHandler

	// All of our current sockets.
	socketsMu sync.Mutex
	socketMap map[*Socket]struct{}

	// event lock.
	eventMu sync.Mutex
}

// NewHandler creates a new live handler.
func NewHandler(store SessionStore, configs ...HandlerConfig) (*Handler, error) {
	h := &Handler{
		sessionStore:     store,
		emitter:          make(chan HandlerEvent),
		broadcastLimiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
		eventHandlers:    make(map[string]EventHandler),
		selfHandlers:     make(map[string]EventHandler),
		Mount: func(ctx context.Context, hd *Handler, r *http.Request, c *Socket, connected bool) (interface{}, error) {
			return nil, nil
		},
		Render: func(ctx context.Context, data interface{}) (io.Reader, error) {
			return nil, ErrNoRenderer
		},
		Error: func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
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

// StartHandler run a handler so that it's events can be dealt with.
// This is called by `NewHandler` so is only required if you are manually
// creating a handler.
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
// h.Self(socket, msg) will be handled here.
func (h *Handler) HandleSelf(t string, handler EventHandler) {
	h.selfHandlers[t] = handler
}

// HandleParams handles a URL query parameter change. This is useful for handling
// things like pagincation, or some filtering.
func (h *Handler) HandleParams(handler EventHandler) {
	h.paramsHandler = handler
}

// serveHTTP serve an http request to the view.
func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	// Get session.
	session, err := h.sessionStore.Get(r)
	if err != nil {
		h.Error(r.Context(), w, r, err)
		return
	}

	// Get socket.
	sock := NewSocket(r.Context(), session)

	// Run mount, this generates the data for the page we are on.
	if err := sock.mount(r.Context(), h, r, false); err != nil {
		h.Error(r.Context(), w, r, err)
		return
	}

	// Handle any query parameters that are on the page.
	if err := sock.params(r.Context(), h, r, false); err != nil {
		h.Error(r.Context(), w, r, err)
		return
	}

	// Render the HTML to display the page.
	if err := sock.render(r.Context(), h); err != nil {
		h.Error(r.Context(), w, r, err)
		return
	}

	var rendered bytes.Buffer
	html.Render(&rendered, sock.currentRender)

	if err := h.sessionStore.Save(w, r, session); err != nil {
		h.Error(r.Context(), w, r, err)
		return
	}

	w.WriteHeader(200)
	io.Copy(w, &rendered)
}

// serveWS serve a websocket request to the view.
func (h *Handler) serveWS(w http.ResponseWriter, r *http.Request) {
	// Get the session from the http request.
	session, err := h.sessionStore.Get(r)
	if err != nil {
		h.Error(r.Context(), w, r, err)
		return
	}

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		h.Error(r.Context(), w, r, err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "")
	writeTimeout(r.Context(), time.Second*5, c, Event{T: EventConnect})
	{
		err := h._serveWS(r, session, c)
		if errors.Is(err, context.Canceled) {
			return
		}
		switch websocket.CloseStatus(err) {
		case websocket.StatusNormalClosure:
			return
		case websocket.StatusGoingAway:
			return
		default:
			log.Println(fmt.Errorf("ws closed with status (%d): %w", websocket.CloseStatus(err), err))
			return
		}
	}
}

type eventError struct {
	Source Event  `json:"source"`
	Err    string `json:"err"`
}

// _serveWS implement the logic for a web socket connection.
func (h *Handler) _serveWS(r *http.Request, session Session, c *websocket.Conn) error {
	ctx := r.Context()
	// Get the sessions socket and register it with the server.
	sock := NewSocket(ctx, session)
	sock.assignWS(c)
	h.addSocket(sock)
	defer h.deleteSocket(sock)

	// Internal errors.
	internalErrors := make(chan error)

	// Event errors.
	eventErrors := make(chan eventError)

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
					if err := h.handleParams(sock, m); err != nil {
						switch {
						case errors.Is(err, ErrNoEventHandler):
							log.Println("event error", m, err)
						default:
							eventErrors <- eventError{Source: m, Err: err.Error()}
						}
					}
				default:
					if err := h.handleEvent(m.T, sock, m); err != nil {
						switch {
						case errors.Is(err, ErrNoEventHandler):
							log.Println("event error", m, err)
						default:
							eventErrors <- eventError{Source: m, Err: err.Error()}
						}
					}
				}
				if err := sock.render(ctx, h); err != nil {
					internalErrors <- fmt.Errorf("socket handle error: %w", err)
				}
				sock.Send(Event{T: EventAck, ID: m.ID})
			case websocket.MessageBinary:
				log.Println("binary messages unhandled")
			}
		}
		close(internalErrors)
		close(eventErrors)
	}()

	// Run mount again now that eh socket is connected, passing true indicating
	// a connection has been made.
	if err := sock.mount(ctx, h, r, true); err != nil {
		return fmt.Errorf("socket mount error: %w", err)
	}

	// Run params again now that the socket is connected.
	if err := sock.params(r.Context(), h, r, true); err != nil {
		return fmt.Errorf("socket params error: %w", err)
	}

	// Run render now that we are connected for the first time and we have just
	// mounted again. This will generate and send any patches if there have
	// been changes.
	if err := sock.render(ctx, h); err != nil {
		return fmt.Errorf("socket render error: %w", err)
	}

	// Send events to the websocket connection.
	for {
		select {
		case msg := <-sock.msgs:
			if err := writeTimeout(ctx, time.Second*5, c, msg); err != nil {
				return fmt.Errorf("writing to socket error: %w", err)
			}
		case ee := <-eventErrors:
			if err := writeTimeout(ctx, time.Second*5, c, Event{T: EventError, Data: ee}); err != nil {
				return fmt.Errorf("writing to socket error: %w", err)
			}
		case err := <-internalErrors:
			if err != nil {
				if err := writeTimeout(ctx, time.Second*5, c, Event{T: EventError, Data: err.Error()}); err != nil {
					return fmt.Errorf("writing to socket error: %w", err)
				}
			}
			// Something catastrophic has happened.
			return fmt.Errorf("read error: %w", err)
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

	// Clear scoped event handlers.
	for id := range h.eventHandlers {
		if strings.HasPrefix(id, sock.Session.ID) {
			delete(h.eventHandlers, id)
		}
	}
	// Clear scoped self handlers
	for id := range h.selfHandlers {
		if strings.HasPrefix(id, sock.Session.ID) {
			delete(h.selfHandlers, id)
		}
	}
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
		return err
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

// handleParams on params change run the handler.
func (h *Handler) handleParams(sock *Socket, msg Event) error {
	params, err := msg.Params()
	if err != nil {
		return fmt.Errorf("received params message and could not extract params: %w", err)
	}

	data, err := h.paramsHandler(sock, params)
	if err != nil {
		return fmt.Errorf("view params handler error: %w", err)
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

// hasSocket check a socket is there error if it isn't connected or
// doensn't exist.
func (h *Handler) hasSocket(s *Socket) error {
	h.socketsMu.Lock()
	defer h.socketsMu.Unlock()
	_, ok := h.socketMap[s]
	if !ok {
		return ErrNoSocket
	}
	return nil
}

func handleEmmittedEvent(h *Handler, he HandlerEvent) {
	// If the socket is nil, this is broadcast message.
	if he.S == nil {
		sockets := h.sockets()
		for _, socket := range sockets {
			_handleEmittedEvent(h, he, socket)
		}
	} else {
		if err := h.hasSocket(he.S); err != nil {
			return
		}
		_handleEmittedEvent(h, he, he.S)
	}
}

func _handleEmittedEvent(h *Handler, he HandlerEvent, socket *Socket) {
	if err := h.handleSelf(he.Msg.T, socket, he.Msg); err != nil {
		log.Println("server event error", err)
	}
	if err := socket.render(context.Background(), h); err != nil {
		log.Println("socket handleView error", err)
	}
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
