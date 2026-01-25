package live

import (
	"context"
	"errors"
	"time"
)

var ErrNoState = errors.New("no state found for socket ID")

type SocketState struct {
	Render []byte
	Data   any
}

type SocketStateStore interface {
	Get(SocketID) (SocketState, error)
	Set(SocketID, SocketState, time.Duration) error
	Delete(SocketID) error
}

var _ SocketStateStore = &MemorySocketStateStore{}

// MemorySocketStateStore an in memory store.
type MemorySocketStateStore struct {
	janitorFrequency time.Duration

	gets  chan mssGetop
	sets  chan mssSetop
	dels  chan mssDelop
	clean chan bool
}

func NewMemorySocketStateStore(ctx context.Context) *MemorySocketStateStore {
	m := &MemorySocketStateStore{
		janitorFrequency: 5 * time.Second,
		gets:             make(chan mssGetop),
		sets:             make(chan mssSetop),
		dels:             make(chan mssDelop),
		clean:            make(chan bool),
	}
	go m.operate(ctx)
	go m.janitor(ctx)
	return m
}

func (m *MemorySocketStateStore) Get(ID SocketID) (SocketState, error) {
	op := mssGetop{
		ID:   ID,
		resp: make(chan SocketState),
		err:  make(chan error),
	}
	m.gets <- op
	select {
	case state := <-op.resp:
		return state, nil
	case err := <-op.err:
		return SocketState{}, err
	}
}

func (m *MemorySocketStateStore) Set(ID SocketID, state SocketState, ttl time.Duration) error {
	op := mssSetop{
		ID:      ID,
		State:   state,
		StaleAt: time.Now().Add(ttl),
		resp:    make(chan bool),
		err:     make(chan error),
	}
	m.sets <- op
	select {
	case <-op.resp:
		return nil
	case err := <-op.err:
		return err
	}
}

func (m *MemorySocketStateStore) Delete(ID SocketID) error {
	op := mssDelop{
		ID:   ID,
		resp: make(chan bool),
		err:  make(chan error),
	}
	m.dels <- op
	select {
	case <-op.resp:
		return nil
	case err := <-op.err:
		return err
	}
}

type mss struct {
	entry time.Time
	stale time.Time
	state SocketState
}

type mssGetop struct {
	ID SocketID

	resp chan SocketState
	err  chan error
}

type mssSetop struct {
	ID      SocketID
	State   SocketState
	StaleAt time.Time

	resp chan bool
	err  chan error
}

type mssDelop struct {
	ID SocketID

	resp chan bool
	err  chan error
}

func (m *MemorySocketStateStore) operate(ctx context.Context) {
	store := map[SocketID]mss{}
	for {
		select {
		case get := <-m.gets:
			ss, ok := store[get.ID]
			if !ok {
				get.err <- ErrNoState
			} else {
				get.resp <- ss.state
			}
		case set := <-m.sets:
			store[set.ID] = mss{
				entry: time.Now(),
				stale: set.StaleAt,
				state: set.State,
			}
			set.resp <- true
		case del := <-m.dels:
			delete(store, del.ID)
			del.resp <- true
		case <-m.clean:
			now := time.Now()
			for k, v := range store {
				if now.Before(v.stale) {
					continue
				}
				delete(store, k)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (m *MemorySocketStateStore) janitor(ctx context.Context) {
	janitor := time.NewTicker(m.janitorFrequency)
	for {
		select {
		case <-janitor.C:
			m.clean <- true
		case <-ctx.Done():
			return
		}
	}
}
