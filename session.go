package live

import (
	"encoding/gob"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/rs/xid"
)

// sessionID the key to access the live session ID.
const sessionID string = "_lsid"

// sessionCookie the name of the session cookie.
const sessionCookie string = "_ls"

// SessionStore handles storing and retrieving sessions.
type SessionStore interface {
	Get(*http.Request) (Session, error)
	Save(http.ResponseWriter, *http.Request, Session) error
}

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

// CookieStore a `gorilla/sessions` based cookie store.
type CookieStore struct {
	Store       *sessions.CookieStore
	sessionName string // session name.
}

// NewCookieStore create a new `gorilla/sessions` based cookie store.
func NewCookieStore(sessionName string, keyPairs ...[]byte) *CookieStore {
	s := sessions.NewCookieStore(keyPairs...)
	s.Options.HttpOnly = true
	s.Options.Secure = false
	s.Options.SameSite = http.SameSiteStrictMode

	return &CookieStore{
		Store:       s,
		sessionName: sessionName,
	}
}

// Get get a session.
func (c CookieStore) Get(r *http.Request) (Session, error) {
	var sess Session
	session, err := c.Store.Get(r, c.sessionName)
	if err != nil {
		return NewSession(), err
	}
	vals, ok := session.Values[sessionCookie]
	if !ok {
		// Create new connection.
		ns := NewSession()
		sess = ns
	} else {
		sess, ok = vals.(Session)
		if !ok {
			// Create new session and set.
			ns := NewSession()
			sess = ns
		}
	}
	return sess, nil
}

// Save a session.
func (c CookieStore) Save(w http.ResponseWriter, r *http.Request, session Session) error {
	s, err := c.Store.Get(r, c.sessionName)
	if err != nil {
		return err
	}
	s.Values[sessionCookie] = session
	return s.Save(r, w)
}
