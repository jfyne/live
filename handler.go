package live

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

var _ Handler = &BaseHandler{}

// MountHandler the func that is called by a handler to gather data to
// be rendered in a template. This is called on first GET and then later when
// the web socket first connects. It should return the state to be maintained
// in the socket.
type MountHandler func(ctx context.Context, c Socket) (interface{}, error)

// RenderHandler the func that is called to render the current state of the
// data for the socket.
type RenderHandler func(ctx context.Context, data interface{}) (io.Reader, error)

// ErrorHandler if an error occurs during the mount and render cycle
// a handler of this type will be called.
type ErrorHandler func(ctx context.Context, err error)

// HandlerConfig applies config to a handler.
type HandlerConfig func(h Handler) error

// BroadcastHandler a way for processes to communicate.
type BroadcastHandler func(ctx context.Context, h Handler, msg Event)

// Handler.
type Handler interface {
	// HandleMount handles initial setup on first request, and then later when
	// the socket first connets.
	HandleMount(handler MountHandler)
	// HandleRender used to set the render method for the handler.
	HandleRender(handler RenderHandler)
	// HandleError for when an error occurs.
	HandleError(handler ErrorHandler)
	// HandleEvent handles an event that comes from the client. For example a click
	// from `live-click="myevent"`.
	HandleEvent(t string, handler EventHandler)
	// HandleSelf handles an event that comes from the server side socket. For example calling
	// h.Self(socket, msg) will be handled here.
	HandleSelf(t string, handler EventHandler)
	// HandleParams handles a URL query parameter change. This is useful for handling
	// things like pagincation, or some filtering.
	HandleParams(handler EventHandler)
	// HandleBroadcast allows overriding the broadcast functionality.
	HandleBroadcast(handler BroadcastHandler)

	// Broadcast send a message to all sockets connected to this handler.
	Broadcast(event string, data interface{}) error

	// Implementation methods.

	// Mount a user should provide the mount function. This is what
	// is called on initial GET request and later when the websocket connects.
	// Data to render the handler should be fetched here and returned.
	Mount() MountHandler
	// Params called to handle any incoming paramters after mount.
	Params() []EventHandler
	// Render is called to generate the HTML of a Socket. It is defined
	// by default and will render any template provided.
	Render() RenderHandler
	// Error is called when an error occurs during the mount and render
	// stages of the handler lifecycle.
	Error() ErrorHandler
	// AddSocket add a socket to the handler.
	AddSocket(sock Socket)
	// DeleteSocket remove a socket from the handler.
	DeleteSocket(sock Socket)
	// CallParams on params change run the handler.
	CallParams(ctx context.Context, sock Socket, msg Event) error
	// CallEvent route an event to the correct handler.
	CallEvent(ctx context.Context, t string, sock Socket, msg Event) error

	// self sends a message to the socket on this handler.
	self(ctx context.Context, sock Socket, msg Event)
}

// BaseHandler handles live inner workings.
type BaseHandler struct {
	// mountHandler a user should provide the mount function. This is what
	// is called on initial GET request and later when the websocket connects.
	// Data to render the handler should be fetched here and returned.
	mountHandler MountHandler
	// Render is called to generate the HTML of a Socket. It is defined
	// by default and will render any template provided.
	renderHandler RenderHandler
	// Error is called when an error occurs during the mount and render
	// stages of the handler lifecycle.
	errorHandler ErrorHandler
	// broadcast handle a broadcast.
	broadcastHandler BroadcastHandler

	// broadcastLimiter limit broadcast rate.
	broadcastLimiter *rate.Limiter

	// eventHandlers the map of client event handlers.
	eventHandlers map[string]EventHandler

	// selfHandlers the map of handler event handlers.
	selfHandlers map[string]EventHandler

	// paramsHandlers a slice of handlers which respond to a change in URL parameters.
	paramsHandlers []EventHandler

	// All of our current sockets.
	socketsMu sync.Mutex
	socketMap map[SocketID]Socket

	// event lock.
	eventMu sync.Mutex

	// ignoreFaviconRequest setting to ignore requests for /favicon.ico.
	ignoreFaviconRequest bool
}

// NewBaseHandler creates a new base handler.
func NewBaseHandler(configs ...HandlerConfig) (*BaseHandler, error) {
	h := &BaseHandler{
		broadcastLimiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
		eventHandlers:    make(map[string]EventHandler),
		selfHandlers:     make(map[string]EventHandler),
		mountHandler: func(ctx context.Context, s Socket) (interface{}, error) {
			return nil, nil
		},
		renderHandler: func(ctx context.Context, data interface{}) (io.Reader, error) {
			return nil, ErrNoRenderer
		},
		errorHandler: func(ctx context.Context, err error) {
			w := Writer(ctx)
			if w != nil {
				w.WriteHeader(500)
				w.Write([]byte(err.Error()))
			}
		},
		socketMap:            make(map[SocketID]Socket),
		paramsHandlers:       []EventHandler{},
		ignoreFaviconRequest: true,
		broadcastHandler: func(ctx context.Context, h Handler, msg Event) {
			h.self(ctx, nil, msg)
		},
	}
	for _, conf := range configs {
		if err := conf(h); err != nil {
			return nil, fmt.Errorf("could not apply config: %w", err)
		}
	}
	return h, nil
}

func (h *BaseHandler) HandleMount(f MountHandler) {
	h.mountHandler = f
}
func (h *BaseHandler) HandleRender(f RenderHandler) {
	h.renderHandler = f
}
func (h *BaseHandler) HandleError(f ErrorHandler) {
	h.errorHandler = f
}
func (h *BaseHandler) HandleBroadcast(f BroadcastHandler) {
	h.broadcastHandler = f
}

func (h *BaseHandler) Mount() MountHandler {
	return h.mountHandler
}

func (h *BaseHandler) Params() []EventHandler {
	return h.paramsHandlers
}

func (h *BaseHandler) Render() RenderHandler {
	return h.renderHandler
}

func (h *BaseHandler) Error() ErrorHandler {
	return h.errorHandler
}

// Broadcast send a message to all sockets connected to this handler.
func (h *BaseHandler) Broadcast(event string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not encode data for broadcast: %w", err)
	}
	e := Event{T: event, Data: payload}
	ctx := context.Background()
	h.broadcastLimiter.Wait(ctx)
	h.broadcastHandler(ctx, h, e)
	return nil
}

// HandleEvent handles an event that comes from the client. For example a click
// from `live-click="myevent"`.
func (h *BaseHandler) HandleEvent(t string, handler EventHandler) {
	h.eventHandlers[t] = handler
}

// HandleSelf handles an event that comes from the server side socket. For example calling
// h.Self(socket, msg) will be handled here.
func (h *BaseHandler) HandleSelf(t string, handler EventHandler) {
	h.selfHandlers[t] = handler
}

// HandleParams handles a URL query parameter change. This is useful for handling
// things like pagincation, or some filtering.
func (h *BaseHandler) HandleParams(handler EventHandler) {
	h.paramsHandlers = append(h.paramsHandlers, handler)
}

// self sends a message to the socket on this handler.
func (h *BaseHandler) self(ctx context.Context, sock Socket, msg Event) {
	// If the socket is nil, this is broadcast message.
	if sock == nil {
		sockets := h.sockets()
		for _, socket := range sockets {
			h.handleEmittedEvent(ctx, socket, msg)
		}
	} else {
		if err := h.hasSocket(sock); err != nil {
			return
		}
		h.handleEmittedEvent(ctx, sock, msg)
	}
}

func (h *BaseHandler) handleEmittedEvent(ctx context.Context, s Socket, msg Event) {
	if err := h.handleSelf(ctx, msg.T, s, msg); err != nil {
		log.Println("server event error", err)
	}
	render, err := RenderSocket(ctx, h, s)
	if err != nil {
		log.Println("socket handleView error", err)
	}
	s.UpdateRender(render)
}

// AddSocket add a socket to the handler.
func (h *BaseHandler) AddSocket(sock Socket) {
	h.socketsMu.Lock()
	defer h.socketsMu.Unlock()
	h.socketMap[sock.ID()] = sock
}

// DeleteSocket remove a socket from the handler.
func (h *BaseHandler) DeleteSocket(sock Socket) {
	h.socketsMu.Lock()
	defer h.socketsMu.Unlock()
	delete(h.socketMap, sock.ID())
}

// CallEvent route an event to the correct handler.
func (h *BaseHandler) CallEvent(ctx context.Context, t string, sock Socket, msg Event) error {
	handler, ok := h.eventHandlers[t]
	if !ok {
		return fmt.Errorf("no event handler for %s: %w", t, ErrNoEventHandler)
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
func (h *BaseHandler) handleSelf(ctx context.Context, t string, sock Socket, msg Event) error {
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

	data, err := handler(ctx, sock, params)
	if err != nil {
		return fmt.Errorf("handler self event handler error [%s]: %w", t, err)
	}
	sock.Assign(data)

	return nil
}

// CallParams on params change run the handler.
func (h *BaseHandler) CallParams(ctx context.Context, sock Socket, msg Event) error {
	params, err := msg.Params()
	if err != nil {
		return fmt.Errorf("received params message and could not extract params: %w", err)
	}

	for _, ph := range h.paramsHandlers {
		data, err := ph(ctx, sock, params)
		if err != nil {
			return fmt.Errorf("handler params handler error: %w", err)
		}
		sock.Assign(data)
	}

	return nil
}

// sockets returns all sockets connected to the handler.
func (h *BaseHandler) sockets() []Socket {
	h.socketsMu.Lock()
	defer h.socketsMu.Unlock()

	sockets := make([]Socket, len(h.socketMap))
	idx := 0
	for _, socket := range h.socketMap {
		sockets[idx] = socket
		idx++
	}
	return sockets
}

// hasSocket check a socket is there error if it isn't connected or
// doensn't exist.
func (h *BaseHandler) hasSocket(s Socket) error {
	h.socketsMu.Lock()
	defer h.socketsMu.Unlock()
	_, ok := h.socketMap[s.ID()]
	if !ok {
		return ErrNoSocket
	}
	return nil
}

// RenderSocket takes the handler and current socket and renders it to html.
func RenderSocket(ctx context.Context, h Handler, s Socket) (*html.Node, error) {
	// Render handler.
	output, err := h.Render()(ctx, s.Assigns())
	if err != nil {
		return nil, fmt.Errorf("render error: %w", err)
	}
	render, err := html.Parse(output)
	if err != nil {
		return nil, fmt.Errorf("html parse error: %w", err)
	}
	shapeTree(render)

	// Get diff
	if s.LatestRender() != nil {
		patches, err := Diff(s.LatestRender(), render)
		if err != nil {
			return nil, fmt.Errorf("diff error: %w", err)
		}
		if len(patches) != 0 {
			s.Send(EventPatch, patches)
		}
	} else {
		anchorTree(render, newAnchorGenerator())
	}

	return render, nil
}
