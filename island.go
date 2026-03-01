// Package live provides a framework for building real-time interactive web applications
// using server-side rendering with live updates via WebSocket or Server-Sent Events.
//
// The v2 architecture uses an islands approach, where individual components (islands)
// maintain their own state and can be independently mounted, updated, and unmounted.
// Each island is a self-contained unit with its own lifecycle, event handlers, and
// rendering logic.
//
// Key concepts:
//
// Islands: Reusable components defined by Island types and registered globally.
// Each island definition includes mount, unmount, render handlers, and event handlers.
//
// Island Instances: Runtime instances of islands with their own isolated state.
// Multiple instances of the same island type can exist simultaneously.
//
// Sessions: Client connections that can host multiple island instances.
// Sessions are transport-agnostic and work with WebSocket, SSE, or other transports.
//
// Engine: The orchestration layer managing sessions, islands, and state persistence.
//
// Transports: Protocol implementations (WebSocket, SSE) for bidirectional communication.
//
// Example usage:
//
//	// Define an island
//	island, err := live.NewIsland("counter",
//		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
//			return &CounterState{Count: 0}, nil
//		}),
//		live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
//			state := rc.State.(*CounterState)
//			return strings.NewReader(fmt.Sprintf("<div>Count: %d</div>", state.Count)), nil
//		}),
//	)
//
//	// Register the island
//	live.RegisterIsland("counter", func() (*live.Island, error) {
//		return island, nil
//	})
//
//	// Create engine and session
//	engine := live.NewIslandEngine(ctx, live.DefaultRegistry(), stateStore)
//	session := live.NewSession(ctx, sessionID, transport)
//	engine.AddSession(session)
//
//	// Mount an island instance
//	instance, err := engine.MountIsland(sessionID, "counter-1", "counter", live.Props{})
package live

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"
)

// Props are the properties passed to an island instance.
// Props are extracted from the island element's attributes.
type Props map[string]any

// String returns the string value for a key, or empty string if not found
// or not a string.
func (p Props) String(key string) string {
	v, ok := p[key]
	if !ok {
		return ""
	}
	out, ok := v.(string)
	if !ok {
		return ""
	}
	return out
}

// Int returns the int value for a key, or 0 if not found or cannot be
// converted to int.
func (p Props) Int(key string) int {
	return mapInt(p, key)
}

// Bool returns the boolean value for a key. Returns true if the value
// is the string "true" or boolean true.
func (p Props) Bool(key string) bool {
	v, ok := p[key]
	if !ok {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true"
	}
	return false
}

// Float32 returns the float32 value for a key, or 0.0 if not found or
// cannot be converted.
func (p Props) Float32(key string) float32 {
	return mapFloat32(p, key)
}

// IslandConfig applies configuration to an Island.
type IslandConfig func(i *Island) error

// IslandMountHandler is called when an island instance is mounted.
// It receives the context, props, and any initial children content.
// Returns the initial state for the island instance.
type IslandMountHandler func(ctx context.Context, props Props, children string) (any, error)

// IslandUnmountHandler is called when an island instance is unmounted.
type IslandUnmountHandler func(ctx context.Context, state any) error

// IslandRenderHandler renders the island's current state.
// It receives the render context containing the state and returns the
// rendered HTML.
type IslandRenderHandler func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error)

// IslandEventHandler handles events from the client.
// Returns the new state after handling the event.
type IslandEventHandler func(ctx context.Context, state any, params Params) (any, error)

// IslandSelfHandler handles self-directed events from the server.
// Returns the new state after handling the event.
type IslandSelfHandler func(ctx context.Context, state any, data any) (any, error)

// IslandRenderContext provides context for rendering an island.
type IslandRenderContext struct {
	// State is the current state of the island instance.
	State any
	// Props are the properties passed to the island.
	Props Props
	// Children is any content nested within the island element.
	Children string
}

// Island defines the behavior and handlers for an island type.
// Islands are registered once and can be instantiated multiple times.
type Island struct {
	// Name is the unique identifier for this island type.
	Name string

	// Mount is called when an island instance is created.
	Mount IslandMountHandler

	// Unmount is called when an island instance is destroyed.
	Unmount IslandUnmountHandler

	// Render generates the HTML for the island's current state.
	Render IslandRenderHandler

	// eventHandlers maps event names to their handlers.
	eventHandlers map[string]IslandEventHandler

	// selfHandlers maps self event names to their handlers.
	selfHandlers map[string]IslandSelfHandler

	// paramsHandler is called when a params event arrives for this island.
	paramsHandler IslandEventHandler

	// errorHandler is called when an event handler returns an error.
	errorHandler func(ctx context.Context, err error) Event

	// eventDelays maps self-event names to their re-delivery delay.
	eventDelays map[string]time.Duration

	// uploadConfigs holds the upload field configurations for this island.
	uploadConfigs []*UploadConfig

	// mu protects concurrent access to handler maps.
	mu sync.RWMutex
}

// NewIsland creates a new Island with the given name and optional configuration.
func NewIsland(name string, configs ...IslandConfig) (*Island, error) {
	i := &Island{
		Name:          name,
		eventHandlers: make(map[string]IslandEventHandler),
		selfHandlers:  make(map[string]IslandSelfHandler),
		eventDelays:   make(map[string]time.Duration),
		errorHandler:  defaultErrorHandler,
		Mount: func(ctx context.Context, props Props, children string) (any, error) {
			return nil, nil
		},
		Unmount: func(ctx context.Context, state any) error {
			return nil
		},
		Render: func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
			return nil, ErrNoRenderer
		},
	}

	for _, config := range configs {
		if err := config(i); err != nil {
			return nil, err
		}
	}

	return i, nil
}

// HandleEvent registers an event handler for the given event name.
// Returns an error if a handler is already registered for that event.
func (i *Island) HandleEvent(event string, handler IslandEventHandler) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, exists := i.eventHandlers[event]; exists {
		return ErrDuplicateEventHandler
	}

	i.eventHandlers[event] = handler
	return nil
}

// HandleSelf registers a self handler for the given event name.
// Returns an error if a handler is already registered for that event.
func (i *Island) HandleSelf(event string, handler IslandSelfHandler) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, exists := i.selfHandlers[event]; exists {
		return ErrDuplicateSelfHandler
	}

	i.selfHandlers[event] = handler
	return nil
}

// GetEventHandler returns the event handler for the given event name.
// Returns ErrNoEventHandler if no handler is registered.
func (i *Island) GetEventHandler(event string) (IslandEventHandler, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	handler, ok := i.eventHandlers[event]
	if !ok {
		return nil, ErrNoEventHandler
	}
	return handler, nil
}

// GetSelfHandler returns the self handler for the given event name.
// Returns ErrNoSelfHandler if no handler is registered.
func (i *Island) GetSelfHandler(event string) (IslandSelfHandler, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	handler, ok := i.selfHandlers[event]
	if !ok {
		return nil, ErrNoSelfHandler
	}
	return handler, nil
}

// HandleParams registers a params handler for the island.
// The handler is called when an EventParams event arrives targeting this island.
// Returns an error if a handler is already registered.
func (i *Island) HandleParams(handler IslandEventHandler) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.paramsHandler != nil {
		return ErrDuplicateEventHandler
	}

	i.paramsHandler = handler
	return nil
}

// GetParamsHandler returns the params handler, or nil if none is registered.
func (i *Island) GetParamsHandler() IslandEventHandler {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.paramsHandler
}

// EventHandlers returns a copy of the event handler names.
func (i *Island) EventHandlers() []string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	names := make([]string, 0, len(i.eventHandlers))
	for name := range i.eventHandlers {
		names = append(names, name)
	}
	return names
}

// SelfHandlers returns a copy of the self handler names.
func (i *Island) SelfHandlers() []string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	names := make([]string, 0, len(i.selfHandlers))
	for name := range i.selfHandlers {
		names = append(names, name)
	}
	return names
}

// WithMount sets the mount handler for the island.
func WithMount(handler IslandMountHandler) IslandConfig {
	return func(i *Island) error {
		i.Mount = handler
		return nil
	}
}

// WithUnmount sets the unmount handler for the island.
func WithUnmount(handler IslandUnmountHandler) IslandConfig {
	return func(i *Island) error {
		i.Unmount = handler
		return nil
	}
}

// WithRender sets the render handler for the island.
func WithRender(handler IslandRenderHandler) IslandConfig {
	return func(i *Island) error {
		i.Render = handler
		return nil
	}
}

// WithErrorHandler sets a custom error handler for the island.
// The handler is called when an event handler returns an error, and must
// return an Event to send back to the client.
func WithErrorHandler(fn func(ctx context.Context, err error) Event) IslandConfig {
	return func(i *Island) error {
		i.errorHandler = fn
		return nil
	}
}

// WithEventDelay configures a re-delivery delay for a named self-event.
// When the engine re-delivers a self-event with this name, it will wait
// for the specified duration before delivery.
func WithEventDelay(event string, delay time.Duration) IslandConfig {
	return func(i *Island) error {
		i.eventDelays[event] = delay
		return nil
	}
}

// WithHandleParams sets a params handler for the island via IslandConfig.
// The handler is called when an EventParams event arrives targeting this island.
func WithHandleParams(handler IslandEventHandler) IslandConfig {
	return func(i *Island) error {
		return i.HandleParams(handler)
	}
}

// GetEventDelay returns the configured delay for the given self-event name.
// Returns (0, false) if no delay is configured for the event.
func (i *Island) GetEventDelay(event string) (time.Duration, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	delay, ok := i.eventDelays[event]
	return delay, ok
}

// GetErrorHandler returns the island's error handler under read lock.
func (i *Island) GetErrorHandler() func(ctx context.Context, err error) Event {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.errorHandler
}

// WithUploadConfig registers an UploadConfig with the island.
// Multiple upload configs can be registered by calling WithUploadConfig
// multiple times, one per named file input.
func WithUploadConfig(config *UploadConfig) IslandConfig {
	return func(i *Island) error {
		i.mu.Lock()
		defer i.mu.Unlock()
		i.uploadConfigs = append(i.uploadConfigs, config)
		return nil
	}
}

// UploadConfigs returns the upload configurations registered with this island.
func (i *Island) UploadConfigs() []*UploadConfig {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.uploadConfigs
}

// defaultErrorHandler is the built-in error handler used when no custom
// handler is configured. It marshals the error message into a JSON payload
// and returns an EventError event.
func defaultErrorHandler(ctx context.Context, err error) Event {
	data, _ := json.Marshal(map[string]string{"err": err.Error()})
	return Event{T: EventError, Data: data}
}

// selfEventSentinel is a non-nil marker used when SendSelf is called with nil data.
// It distinguishes self-events (which always have non-nil SelfData) from client
// events (which always have nil SelfData) when the caller passes nil.
var selfEventSentinel = struct{}{}

// PatchURL sends an EventParams event to the client instructing it to update
// the browser URL query parameters. The values map is URL-encoded as a query
// string (e.g. "page=3&sort=name") and sent as a JSON string in the event data.
//
// PatchURL extracts the session from ctx using the embedded engine and session
// ID. Returns an error if the context does not contain the required values or
// if the session cannot be found.
func PatchURL(ctx context.Context, values map[string]string) error {
	session, err := sessionFromContext(ctx)
	if err != nil {
		return err
	}

	// Encode as URL query string so the client can append directly to pathname.
	urlValues := make(url.Values)
	for k, v := range values {
		urlValues.Set(k, v)
	}
	data, err := json.Marshal(urlValues.Encode())
	if err != nil {
		return fmt.Errorf("PatchURL: failed to marshal values: %w", err)
	}

	return session.Send(Event{T: EventParams, Data: json.RawMessage(data)})
}

// Redirect sends an EventRedirect event to the client instructing the browser
// to navigate to the given URL string.
//
// Redirect extracts the session from ctx using the embedded engine and session
// ID. Returns an error if the context does not contain the required values or
// if the session cannot be found.
func Redirect(ctx context.Context, u string) error {
	session, err := sessionFromContext(ctx)
	if err != nil {
		return err
	}

	data, err := json.Marshal(u)
	if err != nil {
		return fmt.Errorf("Redirect: failed to marshal URL: %w", err)
	}

	return session.Send(Event{T: EventRedirect, Data: json.RawMessage(data)})
}

// SendSelf enqueues a self-directed event onto the island's self-event queue
// stored in ctx. This is a no-op if the queue is not present in the context.
// When data is nil, a sentinel value is used to preserve the distinction between
// self-events and client events (which the router checks via SelfData != nil).
func SendSelf(ctx context.Context, event string, data any) {
	queue := selfEventQueueFromContext(ctx)
	if queue == nil {
		return
	}
	islandID := string(islandIDFromContext(ctx))
	// Use a sentinel when data is nil so that SelfData is never nil for self-events.
	// The session router uses event.SelfData != nil to distinguish self-events from
	// client events; a nil SelfData would cause the tick to be routed as a client event.
	selfData := data
	if selfData == nil {
		selfData = selfEventSentinel
	}
	*queue = append(*queue, Event{T: event, Island: islandID, SelfData: selfData})
}
