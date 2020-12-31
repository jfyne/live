package live

import "errors"

// ErrNoEventHandler returned when a handler has no event handler for that event.
var ErrNoEventHandler = errors.New("view missing event handler")

// ErrMessageMalformed returned when a message could not be parsed correctly.
var ErrMessageMalformed = errors.New("message malformed")

// ErrNoSocket returned when a socket doesn't exist.
var ErrNoSocket = errors.New("no socket")
