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
type MountHandler func(ctx context.Context, view *View, r *http.Request, c *Socket, connected bool) (interface{}, error)

// RenderHandler when the view is asked to render.
type RenderHandler func(ctx context.Context, t *template.Template, data interface{}) (io.Reader, error)

// ViewOption applies config to a view.
type ViewConfig func(v *View) error

// ViewEvent an event sent by the view to the server.
type ViewEvent struct {
	S   *Socket
	Msg Event
}

// View to be handled by the server.
type View struct {
	// session store
	store      sessions.Store
	sessionKey string

	// Template for this view.
	t *template.Template

	// emitter is a channel to send messages back to the server.
	emitter chan ViewEvent

	// broadcastLimiter controls the rate limit applied to broadcasting
	// messages endpoint.
	// Defaults to one publish every 100ms with a burst of 8.
	broadcastLimiter *rate.Limiter

	// eventHandlers the map of event handlers.
	eventHandlers map[ET]EventHandler

	// selfHandlers handle messages send to this view by server-side
	// entities.
	selfHandlers map[ET]EventHandler

	Mount  MountHandler
	Render RenderHandler

	// All of our current sockets.
	socketsMu sync.Mutex
	socketMap map[*Socket]struct{}

	eventMu sync.Mutex
}

// NewView creates a new live view.
func NewView(t *template.Template, sessionKey string, store sessions.Store, configs ...ViewConfig) (*View, error) {
	v := &View{
		t:                t,
		store:            store,
		sessionKey:       sessionKey,
		emitter:          make(chan ViewEvent),
		broadcastLimiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
		eventHandlers:    make(map[ET]EventHandler),
		selfHandlers:     make(map[ET]EventHandler),
		Mount: func(ctx context.Context, view *View, r *http.Request, c *Socket, connected bool) (interface{}, error) {
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
		if err := conf(v); err != nil {
			return nil, fmt.Errorf("could not apply config: %w", err)
		}
	}

	go func(ve *View) {
		for {
			select {
			case m := <-ve.emitter:
				go handleEmmittedEvent(ve, m)
			}
		}
	}(v)

	return v, nil
}

// ServeHTTP serves this view.
func (v *View) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if we are going to upgrade to a webscoket.
	upgrade := false
	for _, header := range r.Header["Upgrade"] {
		if header == "websocket" {
			upgrade = true
			break
		}
	}
	log.Println("upgrade", upgrade, "handling", r.URL)

	if !upgrade {
		// Serve the http version of the view.
		v.serveHTTP(w, r)
		return
	} else {
		// Upgrade to the webscoket version.
		v.serveWS(w, r)
		return
	}
}

// serveHTTP serve an http request to the view.
func (v *View) serveHTTP(w http.ResponseWriter, r *http.Request) {
	// Get session.
	session, err := v.getSession(r)
	if err != nil {
		log.Println("session get err", err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	// Get socket.
	sock := NewSocket(session)

	if err := sock.mount(r.Context(), v, r, false); err != nil {
		log.Println("socket mount err", err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	if err := sock.handleView(r.Context(), v); err != nil {
		log.Println("socket handle view err", err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	var rendered bytes.Buffer
	html.Render(&rendered, sock.currentRender)

	if err := v.saveSession(w, r, session); err != nil {
		log.Println("session save err", err)
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(200)
	io.Copy(w, &rendered)
}

// serveWS serve a websocket request to the view.
func (v *View) serveWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Println("websocket accept error", err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "")

	// Get the session from the http request.
	session, err := v.getSession(r)
	err = v.socket(r.Context(), r, session, c)
	if errors.Is(err, context.Canceled) {
		return
	}
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return
	}
	if err != nil {
		log.Println(err)
		return
	}
}

// socket implement the view for a socket.
func (v *View) socket(ctx context.Context, r *http.Request, session Session, c *websocket.Conn) error {
	// Get the sessions socket and register it with the server.
	sock := NewSocket(session)
	sock.AssignWS(c)
	v.addSocket(sock)
	defer v.deleteSocket(sock)

	if err := sock.mount(ctx, v, r, true); err != nil {
		return fmt.Errorf("socket mount error: %w", err)
	}

	if err := sock.handleView(ctx, v); err != nil {
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
				if err := v.handleEvent(m.T, sock, m); err != nil {
					if !errors.Is(err, ErrNoEventHandler) {
						readError <- err
						break
					} else {
						log.Println("event error", m, err)
					}
				}
				if err := sock.handleView(ctx, v); err != nil {
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
				writeTimeout(ctx, time.Second*5, c, Event{T: ETError, Data: err.Error()})
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

// addSocket add a socket to the view.
func (v *View) addSocket(sock *Socket) {
	v.socketsMu.Lock()
	defer v.socketsMu.Unlock()
	v.socketMap[sock] = struct{}{}
}

// deleteSocket remove a socket from the view.
func (v *View) deleteSocket(sock *Socket) {
	v.socketsMu.Lock()
	defer v.socketsMu.Unlock()
	delete(v.socketMap, sock)
}

// Self sends a message to the socket on this view.
func (v *View) Self(sock *Socket, msg Event) {
	v.emitter <- ViewEvent{
		S:   sock,
		Msg: msg,
	}
}

// Broadcase send a message to all sockets connected to this view.
func (v *View) Broadcast(msg Event) {
	ctx := context.Background()
	v.broadcastLimiter.Wait(ctx)

	v.emitter <- ViewEvent{
		Msg: msg,
	}
}

// HandleEvent handles an event that comes from the client. For example a click
// from `live-click="myevent"`.
func (v *View) HandleEvent(t ET, handler EventHandler) {
	v.eventHandlers[t] = handler
}

// HandleSelf handles an event that comes from the view. For example calling
// view.Self(socket, msg) will be handled here.
func (v *View) HandleSelf(t ET, handler EventHandler) {
	v.selfHandlers[t] = handler
}

// handleEvent route an event to the correct handler.
func (v *View) handleEvent(t ET, sock *Socket, msg Event) error {
	handler, ok := v.eventHandlers[t]
	if !ok {
		return fmt.Errorf("no event handler for %s: %w", t, ErrNoEventHandler)
	}

	params, err := msg.Params()
	if err != nil {
		return fmt.Errorf("recieved message and could not extract params: %w", err)
	}

	data, err := handler(sock, params)
	if err != nil {
		return fmt.Errorf("view event handler error [%s]: %w", t, err)
	}
	sock.Assign(data)

	return nil
}

// handleSelf route an event to the correct handler.
func (v *View) handleSelf(t ET, sock *Socket, msg Event) error {
	v.eventMu.Lock()
	defer v.eventMu.Unlock()

	handler, ok := v.selfHandlers[t]
	if !ok {
		return fmt.Errorf("no self event handler for %s: %w", t, ErrNoEventHandler)
	}

	params, err := msg.Params()
	if err != nil {
		return fmt.Errorf("recieved self message and could not extract params: %w", err)
	}

	data, err := handler(sock, params)
	if err != nil {
		return fmt.Errorf("view self event handler error [%s]: %w", t, err)
	}
	sock.Assign(data)

	return nil
}

// sockets returns all sockets connected to the view.
func (v *View) sockets() []*Socket {
	v.socketsMu.Lock()
	defer v.socketsMu.Unlock()

	sockets := make([]*Socket, len(v.socketMap))
	idx := 0
	for socket := range v.socketMap {
		sockets[idx] = socket
		idx++
	}
	return sockets
}

func handleEmmittedEvent(v *View, ve ViewEvent) {
	// If the socket is nil, this is broadcast message.
	if ve.S == nil {
		sockets := v.sockets()
		for _, socket := range sockets {
			_handleEmittedEvent(v, ve, socket)
		}
	} else {
		_handleEmittedEvent(v, ve, ve.S)
	}
}

func _handleEmittedEvent(v *View, ve ViewEvent, socket *Socket) {
	if err := v.handleSelf(ve.Msg.T, socket, ve.Msg); err != nil {
		log.Println("server event error", err)
	}
	if err := socket.handleView(context.Background(), v); err != nil {
		log.Println("socket handleView error", err)
	}
}

func (v *View) getSession(r *http.Request) (Session, error) {
	var sess Session
	session, err := v.store.Get(r, v.sessionKey)
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

func (v *View) saveSession(w http.ResponseWriter, r *http.Request, session Session) error {
	c, err := v.store.Get(r, v.sessionKey)
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

// WithRootTemplate set the renderer to use a different root template. This changes the views
// Render function.
func WithRootTemplate(rootTemplate string) ViewConfig {
	return func(v *View) error {
		v.Render = func(ctx context.Context, t *template.Template, data interface{}) (io.Reader, error) {
			var buf bytes.Buffer
			if err := t.ExecuteTemplate(&buf, rootTemplate, data); err != nil {
				return nil, err
			}
			return &buf, nil
		}
		return nil
	}
}
