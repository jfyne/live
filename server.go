package live

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/net/html"
	"golang.org/x/time/rate"
	"nhooyr.io/websocket"
)

// Server enables broadcasting to a set of subscribers.
type Server struct {
	// subscriberMessageBuffer controls the max number
	// of messages that can be queued for a subscriber
	// before it is kicked.
	//
	// Defaults to 16.
	subscriberMessageBuffer int

	// publishLimiter controls the rate limit applied to the publish endpoint.
	//
	// Defaults to one publish every 100ms with a burst of 8.
	publishLimiter *rate.Limiter

	// logf controls where logs are sent.
	// Defaults to log.Printf.
	logf func(f string, v ...interface{})

	// serveMux routes the various endpoints to the appropriate handler.
	serveMux mux.Router

	// views the views that this server knows about.
	views map[string]*View

	// session store
	store      sessions.Store
	sessionKey string

	socketsMu sync.Mutex
	sockets   map[*Socket]struct{}
}

// NewServer constructs a Server with the defaults.
func NewServer(sessionKey string, secret []byte) *Server {
	log.Println(sessionKey, string(secret))
	s := &Server{
		subscriberMessageBuffer: 16,
		logf:                    log.Printf,
		sockets:                 make(map[*Socket]struct{}),
		publishLimiter:          rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
		store:                   sessions.NewCookieStore(secret),
		sessionKey:              sessionKey,
		views:                   map[string]*View{},
	}
	s.serveMux.HandleFunc("/live.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/live.js")
	})
	s.serveMux.HandleFunc("/socket", s.socketHandler)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.serveMux.ServeHTTP(w, r)
}

func (s *Server) getSession(r *http.Request) (Session, error) {
	var sess Session
	session, err := s.store.Get(r, s.sessionKey)
	if err != nil {
		return NewSession(), err
	}

	v, ok := session.Values[SessionKey]
	if !ok {
		log.Println("failed to find existing conn")
		// Create new connection.
		ns := NewSession()
		sess = ns
	}
	sess, ok = v.(Session)
	if !ok {
		log.Println("failed to assert conn type")
		// Create new connection and set.
		ns := NewSession()
		sess = ns
	}
	return sess, nil
}

func (s *Server) saveSession(w http.ResponseWriter, r *http.Request, session Session) error {
	c, err := s.store.Get(r, s.sessionKey)
	if err != nil {
		return err
	}
	c.Values[SessionKey] = session
	return c.Save(r, w)
}

func (s *Server) Add(view *View) {
	// Register the view
	s.views[view.path] = view

	s.serveMux.HandleFunc(view.path, func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		// Get session.
		session, err := s.getSession(r)
		if err != nil {
			s.logf("session get err: %w", err)
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}

		// Get connection.
		conn := NewSocket(session)

		// Mount view.
		if err := view.Mount(r.Context(), params, conn, false); err != nil {
			s.logf("mount err: %w", err)
			w.WriteHeader(500)
			return
		}

		// Render view.
		output, err := view.Render(r.Context(), view.t, conn)
		if err != nil {
			log.Println(err)
			s.logf("err: %w", err)
			w.WriteHeader(500)
			return
		}
		node, err := html.Parse(output)
		if err != nil {
			s.logf("err: %w", err)
			w.WriteHeader(500)
			return
		}

		var rendered bytes.Buffer
		html.Render(&rendered, node)

		if err := s.saveSession(w, r, session); err != nil {
			s.logf("session save err: %w", err)
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(200)
		io.Copy(w, &rendered)
	})

	go func(v *View) {
		for {
			select {
			case m := <-v.emitter:
				log.Println(m)
			}
		}
	}(view)
}

// socketHandler accepts the WebSocket connection and then subscribes
// it to all future messages.
func (s *Server) socketHandler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.logf("%v", err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "")

	session, err := s.getSession(r)
	err = s.socket(r.Context(), r.URL, session, c)
	if errors.Is(err, context.Canceled) {
		return
	}
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return
	}
	if err != nil {
		s.logf("%v", err)
		return
	}
}

func (s *Server) socket(ctx context.Context, url *url.URL, session Session, c *websocket.Conn) error {
	con := NewSocket(session)
	con.AssignWS(c)
	s.addSocket(con)
	defer s.deleteSocket(con)

	s.logf("%s connected on %s", session.ID, url.Path)

	readError := make(chan error)
	go func() {
		for {
			t, d, err := c.Read(ctx)
			if err != nil {
				readError <- err
				break
			}
			switch t {
			case websocket.MessageText:
				var m SocketMessage
				if err := json.Unmarshal(d, &m); err != nil {
					readError <- err
					break
				}
				switch m.T {
				case EventListen:
					log.Println("listen event", m)
				}
			case websocket.MessageBinary:
				log.Println("binary messages unhandled")
			}
		}
		close(readError)
	}()

	for {
		select {
		case err := <-readError:
			if err != nil {
				return fmt.Errorf("read error: %w", err)
			}
		case msg := <-con.msgs:
			err := writeTimeout(ctx, time.Second*5, c, msg)
			if err != nil {
				return err
			}
		case <-ctx.Done():
		}
	}
}

func (s *Server) broadcast(msg SocketMessage) {
	s.socketsMu.Lock()
	defer s.socketsMu.Unlock()

	s.publishLimiter.Wait(context.Background())

	for c := range s.sockets {
		select {
		case c.msgs <- msg:
		default:
			go c.closeSlow()
		}
	}
}

// addSocket registers a connection.
func (s *Server) addSocket(c *Socket) {
	s.socketsMu.Lock()
	s.sockets[c] = struct{}{}
	s.socketsMu.Unlock()
}

// deleteSocket deletes the given connection.
func (s *Server) deleteSocket(c *Socket) {
	s.socketsMu.Lock()
	delete(s.sockets, c)
	s.socketsMu.Unlock()
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg SocketMessage) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(&msg)
	if err != nil {
		return err
	}

	return c.Write(ctx, websocket.MessageText, data)
}

// RunServer run a live server.
func RunServer(ls *Server) error {
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		return err
	}
	log.Printf("listening on http://%v", l.Addr())

	s := &http.Server{
		Handler:      ls,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}
	errc := make(chan error, 1)
	go func() {
		errc <- s.Serve(l)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select {
	case err := <-errc:
		log.Printf("failed to serve: %v", err)
	case sig := <-sigs:
		log.Printf("terminating: %v", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return s.Shutdown(ctx)
}
