package live

import (
	"encoding/gob"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/rs/xid"
)

// SessionStore handles storing and retrieving sessions.
type SessionStore interface {
	Get(*http.Request) (Session, error)
	Save(http.ResponseWriter, *http.Request, Session) error
}

// Session what we will actually store across page loads.
type Session struct {
	ID string
}

// NewSession create a new session.
func NewSession() Session {
	return Session{ID: NewID()}
}

// ValueKey type for session keys.
type ValueKey string

// NewID returns a new ID.
func NewID() string {
	return xid.New().String()
}

func init() {
	gob.Register(ValueKey(""))
	gob.Register(Session{})
}

// CookieStore a `gorilla/sessions` based cookie store.
type CookieStore struct {
	store       *sessions.CookieStore
	sessionName string // session name.
}

// NewCookieStore create a new `gorilla/sessions` based cookie store.
func NewCookieStore(sessionName string, keyPairs ...[]byte) *CookieStore {
	s := sessions.NewCookieStore(keyPairs...)
	s.Options.HttpOnly = true
	s.Options.Secure = true
	s.Options.SameSite = http.SameSiteStrictMode

	return &CookieStore{
		store:       s,
		sessionName: sessionName,
	}
}

// Get get a session.
func (c CookieStore) Get(r *http.Request) (Session, error) {
	var sess Session
	session, err := c.store.Get(r, c.sessionName)
	if err != nil {
		return NewSession(), err
	}
	vals, ok := session.Values["s"]
	if !ok {
		// Create new connection.
		ns := NewSession()
		sess = ns
	}
	sess, ok = vals.(Session)
	if !ok {
		// Create new session and set.
		ns := NewSession()
		sess = ns
	}
	return sess, nil
}

// Save a session.
func (c CookieStore) Save(w http.ResponseWriter, r *http.Request, session Session) error {
	s, err := c.store.Get(r, c.sessionName)
	if err != nil {
		return err
	}
	s.Values["s"] = session
	return s.Save(r, w)
}
