package live

import (
	"encoding/gob"

	"github.com/rs/xid"
)

// sessionID the key to access the live session ID.
const sessionID string = "_lsid"

// Session persisted over page loads.
type Session map[string]interface{}

// NewSession create a new session.
func NewSession() Session {
	return map[string]interface{}{
		sessionID: NewID(),
	}
}

// SessionID helper to get the sessions live ID.
func SessionID(session Session) string {
	ID, ok := session[sessionID].(string)
	if !ok {
		return ""
	}
	return ID
}

// NewID returns a new ID.
func NewID() string {
	return xid.New().String()
}

func init() {
	gob.Register(Session{})
}
