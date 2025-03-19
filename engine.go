package live

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"nhooyr.io/websocket"
)

//var _ Engine = &BaseEngine{}

// EngineConfig applies configuration to an engine.
type EngineConfig func(e *Engine) error

// BroadcastHandler a way for processes to communicate.
type BroadcastHandler func(ctx context.Context, e *Engine, msg Event)

//// Engine methods.
//type Engine interface {
//	// Handler takes a handler to configure the lifecycle.
//	Handler(h Handler)
//	// Mount a user should provide the mount function. This is what
//	// is called on initial GET request and later when the websocket connects.
//	// Data to render the handler should be fetched here and returned.
//	Mount() MountHandler
//	// UnmountHandler the func that is called by a handler to report that a connection
//	// is closed. This is called on websocket close. Can be used to track number of
//	// connected users.
//	Unmount() UnmountHandler
//	// Params called to handle any incoming paramters after mount.
//	Params() []EventHandler
//	// Render is called to generate the HTML of a Socket. It is defined
//	// by default and will render any template provided.
//	Render() RenderHandler
//	// Error is called when an error occurs during the mount and render
//	// stages of the handler lifecycle.
//	Error() ErrorHandler
//	// AddSocket add a socket to the engine.
//	AddSocket(sock Socket)
//	// GetSocket from a session get an already connected
//	// socket.
//	GetSocket(session Session) (Socket, error)
//	// DeleteSocket remove a socket from the engine.
//	DeleteSocket(sock Socket)
//	// CallParams on params change run the handlers.
//	CallParams(ctx context.Context, sock Socket, msg Event) error
//	// CallEvent route an event to the correct handler.
//	CallEvent(ctx context.Context, t string, sock Socket, msg Event) error
//	// HandleBroadcast allows overriding the broadcast functionality.
//	HandleBroadcast(handler BroadcastHandler)
//	// Broadcast send a message to all sockets connected to this engine.
//	Broadcast(event string, data interface{}) error
//
//	// self sends a message to the socket on this engine.
//	self(ctx context.Context, sock Socket, msg Event)
//}

// Engine handles live inner workings.
type Engine struct {
	// Handler implements all the developer defined logic.
	Handler *Handler

	// BroadcastLimiter limit broadcast ratehandler.
	BroadcastLimiter *rate.Limiter
	// broadcast handle a broadcast.
	BroadcastHandler BroadcastHandler
	// All of our current sockets.
	socketsMu sync.Mutex
	socketMap map[SocketID]*Socket

	// event lock.
	eventMu sync.Mutex

	// IgnoreFaviconRequest setting to ignore requests for /favicon.ico.
	IgnoreFaviconRequest bool

	// MaxUploadSize the maximum upload size in bytes to allow. This defaults
	// too 100MB.
	MaxUploadSize int64

	// UploadStagingLocation where uploads are stored before they are consumed. This defaults
	// too the default OS temp directory.
	UploadStagingLocation string

	acceptOptions *websocket.AcceptOptions
	sessionStore  HttpSessionStore
}

// NewHttpHandler serve the handler.
func NewHttpHandler(s HttpSessionStore, h *Handler, configs ...EngineConfig) *Engine {
	const maxUploadSize = 100 * 1024 * 1024
	e := &Engine{
		BroadcastLimiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
		BroadcastHandler: func(ctx context.Context, h *Engine, msg Event) {
			h.self(ctx, nil, msg)
		},
		socketMap:            make(map[SocketID]*Socket),
		IgnoreFaviconRequest: true,
		MaxUploadSize:        maxUploadSize,
		Handler:              h,
		sessionStore:         s,
	}
	for _, conf := range configs {
		if err := conf(e); err != nil {
			slog.Warn(fmt.Sprintf("could not apply config to engine: %s", err))
		}
	}
	return e
}

//func (e *Engine) Handler(hand *Handler) {
//	e.handler = hand
//}
//func (e *Engine) HandleBroadcast(f BroadcastHandler) {
//	e.broadcastHandler = f
//}

//func (e *Engine) Mount() MountHandler {
//	return e.Handler.MountHandler
//}
//
//func (e *Engine) Unmount() UnmountHandler {
//	return e.Handler.UnmountHandler
//}
//
//func (e *Engine) Render() RenderHandler {
//	return e.Handler.RenderHandler
//}
//
//func (e *Engine) Error() ErrorHandler {
//	return e.Handler.ErrorHandler
//}

// Broadcast send a message to all sockets connected to this engine.
func (e *Engine) Broadcast(event string, data interface{}) error {
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
		for _, socket := range e.socketMap {
			e.handleEmittedEvent(ctx, socket, msg)
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
		log.Println("server event error", err)
	}
	render, err := RenderSocket(ctx, e, s)
	if err != nil {
		log.Println("socket handleView error", err)
	}
	s.UpdateRender(render)
}

// AddSocket add a socket to the engine.
func (e *Engine) AddSocket(sock *Socket) {
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()
	e.socketMap[sock.ID()] = sock
}

// GetSocket get a socket from a session.
func (e *Engine) GetSocket(session Session) (*Socket, error) {
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()
	for _, s := range e.socketMap {
		if SessionID(session) == SessionID(s.Session()) {
			return s, nil
		}
	}
	return nil, ErrNoSocket
}

// DeleteSocket remove a socket from the engine.
func (e *Engine) DeleteSocket(sock *Socket) {
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()
	delete(e.socketMap, sock.ID())
	err := e.Handler.UnmountHandler(sock)
	if err != nil {
		log.Println("socket unmount error", err)
	}
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
	e.eventMu.Lock()
	defer e.eventMu.Unlock()

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
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()
	_, ok := e.socketMap[s.ID()]
	if !ok {
		return ErrNoSocket
	}
	return nil
}
