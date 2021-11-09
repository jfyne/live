package live

import "net/http"

// TestStore a test session store.
type TestStore struct {
	s Session
}

// NewTestStore return a new test store.
func NewTestStore(ID string) *TestStore {
	t := &TestStore{
		s: NewSession(),
	}
	t.s[sessionID] = ID
	return t
}

// Get a session.
func (t TestStore) Get(r *http.Request) (Session, error) {
	return t.s, nil
}

// Save a session.
func (t *TestStore) Save(w http.ResponseWriter, r *http.Request, session Session) error {
	t.s = session
	return nil
}
