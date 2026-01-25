package live

import "errors"

// ErrNoRenderer returned when no renderer has been set on the handler.
var ErrNoRenderer = errors.New("no renderer has been set on the handler")

// ErrNoEventHandler returned when a handler has no event handler for that event.
var ErrNoEventHandler = errors.New("view missing event handler")

// ErrMessageMalformed returned when a message could not be parsed correctly.
var ErrMessageMalformed = errors.New("message malformed")

// ErrNoSocket returned when a socket doesn't exist.
var ErrNoSocket = errors.New("no socket")

// ErrNotImplemented returned when an interface has not been implemented correctly.
var ErrNotImplemented = errors.New("not implemented")

// ErrDuplicateEventHandler returned when attempting to register an event handler
// for an event that already has a handler.
var ErrDuplicateEventHandler = errors.New("duplicate event handler")

// ErrDuplicateSelfHandler returned when attempting to register a self handler
// for an event that already has a handler.
var ErrDuplicateSelfHandler = errors.New("duplicate self handler")

// ErrNoSelfHandler returned when a handler has no self handler for that event.
var ErrNoSelfHandler = errors.New("island missing self handler")

// ErrIslandNotFound returned when an island type is not found in the registry.
var ErrIslandNotFound = errors.New("island not found")

// ErrIslandAlreadyRegistered returned when attempting to register an island
// with a name that is already registered.
var ErrIslandAlreadyRegistered = errors.New("island already registered")
