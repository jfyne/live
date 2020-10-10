package live

import (
	"encoding/gob"

	"github.com/rs/xid"
)

// Session what we will actually store across page loads.
type Session struct {
	ID string
}

// NewSession create a new session.
func NewSession() Session {
	return Session{ID: NewID()}
}

// SessionKey key to find items stored in the session.
type ValueKey string

// Session keys.
const (
	SessionKey ValueKey = "s"
)

// NewID returns a new ID.
func NewID() string {
	return xid.New().String()
}

func init() {
	gob.Register(ValueKey(""))
	gob.Register(Session{})
}
