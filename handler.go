package live

import (
	"context"
	"fmt"
	"io"
	"log"
)

var _ Handler = &BaseHandler{}

// HandlerConfig applies config to a handler.
type HandlerConfig func(h Handler) error

// MountHandler the func that is called by a handler to gather data to
// be rendered in a template. This is called on first GET and then later when
// the web socket first connects. It should return the state to be maintained
// in the socket.
type MountHandler func(ctx context.Context, c Socket) (interface{}, error)

// RenderHandler the func that is called to render the current state of the
// data for the socket.
type RenderHandler func(ctx context.Context, rc *RenderContext) (io.Reader, error)

// ErrorHandler if an error occurs during the mount and render cycle
// a handler of this type will be called.
type ErrorHandler func(ctx context.Context, err error)

// EventHandler a function to handle events, returns the data that should
// be set to the socket after handling.
type EventHandler func(context.Context, Socket, Params) (interface{}, error)

// SelfHandler a function to handle self events, returns the data that should
// be set to the socket after handling.
type SelfHandler func(context.Context, Socket, interface{}) (interface{}, error)

// Handler methods.
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
	HandleSelf(t string, handler SelfHandler)
	// HandleParams handles a URL query parameter change. This is useful for handling
	// things like pagincation, or some filtering.
	HandleParams(handler EventHandler)

	getMount() MountHandler
	getRender() RenderHandler
	getError() ErrorHandler
	getEvent(t string) (EventHandler, error)
	getSelf(t string) (SelfHandler, error)
	getParams() []EventHandler
}

// BaseHandler.
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
	// eventHandlers the map of client event handlers.
	eventHandlers map[string]EventHandler
	// selfHandlers the map of handler event handlers.
	selfHandlers map[string]SelfHandler
	// paramsHandlers a slice of handlers which respond to a change in URL parameters.
	paramsHandlers []EventHandler
}

// NewHandler sets up a base handler for live.
func NewHandler(configs ...HandlerConfig) *BaseHandler {
	h := &BaseHandler{
		eventHandlers:  make(map[string]EventHandler),
		selfHandlers:   make(map[string]SelfHandler),
		paramsHandlers: []EventHandler{},
		mountHandler: func(ctx context.Context, s Socket) (interface{}, error) {
			return nil, nil
		},
		renderHandler: func(ctx context.Context, rc *RenderContext) (io.Reader, error) {
			return nil, ErrNoRenderer
		},
		errorHandler: func(ctx context.Context, err error) {
			w := Writer(ctx)
			if w != nil {
				w.WriteHeader(500)
				w.Write([]byte(err.Error()))
			}
		},
	}
	for _, conf := range configs {
		if err := conf(h); err != nil {
			log.Println("warning:", fmt.Errorf("could not apply config to handler: %w", err))
		}
	}
	return h
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

// HandleEvent handles an event that comes from the client. For example a click
// from `live-click="myevent"`.
func (h *BaseHandler) HandleEvent(t string, handler EventHandler) {
	h.eventHandlers[t] = handler
}

// HandleSelf handles an event that comes from the server side socket. For example calling
// h.Self(socket, msg) will be handled here.
func (h *BaseHandler) HandleSelf(t string, handler SelfHandler) {
	h.selfHandlers[t] = handler
}

// HandleParams handles a URL query parameter change. This is useful for handling
// things like pagincation, or some filtering.
func (h *BaseHandler) HandleParams(handler EventHandler) {
	h.paramsHandlers = append(h.paramsHandlers, handler)
}

func (h *BaseHandler) getMount() MountHandler {
	return h.mountHandler
}
func (h *BaseHandler) getRender() RenderHandler {
	return h.renderHandler
}
func (h *BaseHandler) getError() ErrorHandler {
	return h.errorHandler
}
func (h *BaseHandler) getEvent(t string) (EventHandler, error) {
	handler, ok := h.eventHandlers[t]
	if !ok {
		return nil, fmt.Errorf("no event handler for %s: %w", t, ErrNoEventHandler)
	}
	return handler, nil
}
func (h *BaseHandler) getSelf(t string) (SelfHandler, error) {
	handler, ok := h.selfHandlers[t]
	if !ok {
		return nil, fmt.Errorf("no self handler for %s: %w", t, ErrNoEventHandler)
	}
	return handler, nil
}
func (h *BaseHandler) getParams() []EventHandler {
	return h.paramsHandlers
}
