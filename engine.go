package live

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var _ Engine = &BaseEngine{}

// EngineConfig applies configuration to an engine.
type EngineConfig func(e Engine) error

// BroadcastHandler a way for processes to communicate.
type BroadcastHandler func(ctx context.Context, e Engine, msg Event)

// Engine methods.
type Engine interface {
	// Handler takes a handler to configure the lifecycle.
	Handler(h *Handler)
	// Mount a user should provide the mount function. This is what
	// is called on initial GET request and later when the websocket connects.
	// Data to render the handler should be fetched here and returned.
	Mount() MountHandler
	// UnmountHandler the func that is called by a handler to report that a connection
	// is closed. This is called on websocket close. Can be used to track number of
	// connected users.
	Unmount() UnmountHandler
	// Params called to handle any incoming paramters after mount.
	Params() []EventHandler
	// Render is called to generate the HTML of a Socket. It is defined
	// by default and will render any template provided.
	Render() RenderHandler
	// Error is called when an error occurs during the mount and render
	// stages of the handler lifecycle.
	Error() ErrorHandler
	// AddSocket add a socket to the engine.
	AddSocket(sock Socket)
	// GetSocket from a session get an already connected
	// socket.
	GetSocket(session Session) (Socket, error)
	// DeleteSocket remove a socket from the engine.
	DeleteSocket(sock Socket)
	// CallParams on params change run the handlers.
	CallParams(ctx context.Context, sock Socket, msg Event) error
	// CallEvent route an event to the correct handler.
	CallEvent(ctx context.Context, t string, sock Socket, msg Event) error
	// HandleBroadcast allows overriding the broadcast functionality.
	HandleBroadcast(handler BroadcastHandler)
	// Broadcast send a message to all sockets connected to this engine.
	Broadcast(event string, data any) error

	// self sends a message to the socket on this engine.
	self(ctx context.Context, sock Socket, msg Event)
}

// BaseEngine handles live inner workings.
type BaseEngine struct {
	// handler implements all the developer defined logic.
	handler *Handler

	// broadcastLimiter limit broadcast ratehandler.
	broadcastLimiter *rate.Limiter
	// broadcast handle a broadcast.
	broadcastHandler BroadcastHandler
	// All of our current sockets.
	socketsMu sync.Mutex
	socketMap map[SocketID]Socket

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
}

// NewBaseEngine creates a new base engine.
func NewBaseEngine(h *Handler) *BaseEngine {
	const maxUploadSize = 100 * 1024 * 1024
	return &BaseEngine{
		broadcastLimiter: rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
		broadcastHandler: func(ctx context.Context, h Engine, msg Event) {
			h.self(ctx, nil, msg)
		},
		socketMap:            make(map[SocketID]Socket),
		IgnoreFaviconRequest: true,
		MaxUploadSize:        maxUploadSize,
		handler:              h,
	}
}

func (e *BaseEngine) Handler(hand *Handler) {
	e.handler = hand
}
func (e *BaseEngine) HandleBroadcast(f BroadcastHandler) {
	e.broadcastHandler = f
}

func (e *BaseEngine) Mount() MountHandler {
	return e.handler.mountHandler
}

func (e *BaseEngine) Unmount() UnmountHandler {
	return e.handler.getUnmount()
}

func (e *BaseEngine) Params() []EventHandler {
	return e.handler.paramsHandlers
}

func (e *BaseEngine) Render() RenderHandler {
	return e.handler.renderHandler
}

func (e *BaseEngine) Error() ErrorHandler {
	return e.handler.errorHandler
}

// Broadcast send a message to all sockets connected to this engine.
func (e *BaseEngine) Broadcast(event string, data any) error {
	ev := Event{T: event, SelfData: data}
	ctx := context.Background()
	e.broadcastLimiter.Wait(ctx)
	e.broadcastHandler(ctx, e, ev)
	return nil
}

// self sends a message to the socket on this engine.
func (e *BaseEngine) self(ctx context.Context, sock Socket, msg Event) {
	// If the socket is nil, this is broadcast message.
	if sock == nil {
		sockets := e.sockets()
		for _, socket := range sockets {
			e.handleEmittedEvent(ctx, socket, msg)
		}
	} else {
		if err := e.hasSocket(sock); err != nil {
			return
		}
		e.handleEmittedEvent(ctx, sock, msg)
	}
}

func (e *BaseEngine) handleEmittedEvent(ctx context.Context, s Socket, msg Event) {
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
func (e *BaseEngine) AddSocket(sock Socket) {
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()
	e.socketMap[sock.ID()] = sock
}

// GetSocket get a socket from a session.
func (e *BaseEngine) GetSocket(session Session) (Socket, error) {
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
func (e *BaseEngine) DeleteSocket(sock Socket) {
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()
	delete(e.socketMap, sock.ID())
	err := e.Unmount()(sock)
	if err != nil {
		log.Println("socket unmount error", err)
	}
}

// CallEvent route an event to the correct handler.
func (e *BaseEngine) CallEvent(ctx context.Context, t string, sock Socket, msg Event) error {
	handler, err := e.handler.getEvent(t)
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
func (e *BaseEngine) handleSelf(ctx context.Context, t string, sock Socket, msg Event) error {
	e.eventMu.Lock()
	defer e.eventMu.Unlock()

	handler, err := e.handler.getSelf(t)
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
func (e *BaseEngine) CallParams(ctx context.Context, sock Socket, msg Event) error {
	params, err := msg.Params()
	if err != nil {
		return fmt.Errorf("received params message and could not extract params: %w", err)
	}

	for _, ph := range e.handler.paramsHandlers {
		data, err := ph(ctx, sock, params)
		if err != nil {
			return fmt.Errorf("handler params handler error: %w", err)
		}
		sock.Assign(data)
	}

	return nil
}

// sockets returns all sockets connected to the engine.
func (e *BaseEngine) sockets() []Socket {
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()

	sockets := make([]Socket, len(e.socketMap))
	idx := 0
	for _, socket := range e.socketMap {
		sockets[idx] = socket
		idx++
	}
	return sockets
}

// hasSocket check a socket is there error if it isn't connected or
// doesn't exist.
func (e *BaseEngine) hasSocket(s Socket) error {
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()
	_, ok := e.socketMap[s.ID()]
	if !ok {
		return ErrNoSocket
	}
	return nil
}
