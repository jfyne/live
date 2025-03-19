package live

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"golang.org/x/net/html"
	"nhooyr.io/websocket"
)

// var _ Engine = &HttpEngine{}
//var _ Socket = &HttpSocket{}

var _ HttpSessionStore = &CookieStore{}

// sessionCookie the name of the session cookie.
const sessionCookie string = "_ls"

// HttpSessionStore handles storing and retrieving sessions.
type HttpSessionStore interface {
	Get(*http.Request) (Session, error)
	Save(http.ResponseWriter, *http.Request, Session) error
	Clear(http.ResponseWriter, *http.Request) error
}

//// HttpEngine serves live for net/http.
//type HttpEngine struct {
//	acceptOptions *websocket.AcceptOptions
//	sessionStore  HttpSessionStore
//	*Engine
//}

//// WithWebsocketAcceptOptions apply websocket accept options to the HTTP engine.
//func WithWebsocketAcceptOptions(options *websocket.AcceptOptions) EngineConfig {
//	return func(e *HttpEngine) error {
//		if httpEngine, ok := e.(*HttpEngine); ok {
//			httpEngine.acceptOptions = options
//		}
//		return nil
//	}
//}

//// NewHttpHandler returns the net/http handler for live.
//func NewHttpHandler(handler *Handler, configs ...EngineConfig) *Engine {
//	e := NewEngine(handler)
//	for _, conf := range configs {
//		if err := conf(e); err != nil {
//			log.Println("warning:", fmt.Errorf("could not apply config to engine: %w", err))
//		}
//	}
//	return e
//}

// ServeHTTP serves this handler.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		if e.IgnoreFaviconRequest {
			w.WriteHeader(404)
			return
		}
	}

	// Check if we are going to upgrade to a websocket.
	upgrade := false
	for _, header := range r.Header["Upgrade"] {
		if header == "websocket" {
			upgrade = true
			break
		}
	}

	ctx := httpContext(w, r)

	if !upgrade {
		switch r.Method {
		case http.MethodPost:
			e.post(ctx, w, r)
		default:
			e.get(ctx, w, r)
		}
		return
	}

	// Upgrade to the websocket version.
	e.serveWS(ctx, w, r)
}

// post handler.
func (e *Engine) post(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Get session.
	session, err := e.sessionStore.Get(r)
	if err != nil {
		e.Handler.ErrorHandler(ctx, fmt.Errorf("no session found: %w", err))
		return
	}

	// Get socket.
	sock, err := e.GetSocket(session)
	if err != nil {
		e.Handler.ErrorHandler(ctx, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, e.MaxUploadSize)
	if err := r.ParseMultipartForm(e.MaxUploadSize); err != nil {
		e.Handler.ErrorHandler(ctx, fmt.Errorf("could not parse form for uploads: %w", err))
		return
	}

	uploadDir := filepath.Join(e.UploadStagingLocation, string(sock.ID()))
	if e.UploadStagingLocation == "" {
		uploadDir, err = os.MkdirTemp("", string(sock.ID()))
		if err != nil {
			e.Handler.ErrorHandler(ctx, fmt.Errorf("%s upload dir creation failed: %w", sock.ID(), err))
			return
		}
	}

	for _, config := range sock.UploadConfigs() {
		for _, fileHeader := range r.MultipartForm.File[config.Name] {
			u := uploadFromFileHeader(fileHeader)
			sock.AssignUpload(config.Name, u)
			handleFileUpload(e, sock, config, u, uploadDir, fileHeader)

			render, err := RenderSocket(ctx, e, sock)
			if err != nil {
				e.Handler.ErrorHandler(ctx, err)
				return
			}
			sock.UpdateRender(render)
		}
	}
}

func uploadFromFileHeader(fh *multipart.FileHeader) *Upload {
	return &Upload{
		Name: fh.Filename,
		Size: fh.Size,
	}
}

func handleFileUpload(h *Engine, sock *Socket, config *UploadConfig, u *Upload, uploadDir string, fileHeader *multipart.FileHeader) {
	// Check file claims to be within the max size.
	if fileHeader.Size > config.MaxSize {
		u.Errors = append(u.Errors, fmt.Errorf("%s greater than max allowed size of %d", fileHeader.Filename, config.MaxSize))
		return
	}

	// Open the incoming file.
	file, err := fileHeader.Open()
	if err != nil {
		u.Errors = append(u.Errors, fmt.Errorf("could not open %s for upload: %w", fileHeader.Filename, err))
		return
	}
	defer file.Close()

	// Check the actual filetype.
	buff := make([]byte, 512)
	_, err = file.Read(buff)
	if err != nil {
		u.Errors = append(u.Errors, fmt.Errorf("could not check %s for type: %w", fileHeader.Filename, err))
		return
	}
	filetype := http.DetectContentType(buff)
	allowed := false
	for _, a := range config.Accept {
		if filetype == a {
			allowed = true
			break
		}
	}
	if !allowed {
		u.Errors = append(u.Errors, fmt.Errorf("%s filetype is not allowed", fileHeader.Filename))
		return
	}
	u.Type = filetype

	// Rewind to start of the
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		u.Errors = append(u.Errors, fmt.Errorf("%s rewind error: %w", fileHeader.Filename, err))
		return
	}

	f, err := os.Create(filepath.Join(uploadDir, fmt.Sprintf("%d%s", time.Now().UnixNano(), filepath.Ext(fileHeader.Filename))))
	if err != nil {
		u.Errors = append(u.Errors, fmt.Errorf("%s upload file creation failed: %w", fileHeader.Filename, err))
		return
	}
	defer f.Close()
	u.internalLocation = f.Name()
	u.Name = fileHeader.Filename

	written, err := io.Copy(f, io.TeeReader(file, &UploadProgress{Upload: u, Engine: h, Socket: sock}))
	if err != nil {
		u.Errors = append(u.Errors, fmt.Errorf("%s upload failed: %w", fileHeader.Filename, err))
		return
	}
	u.Size = written
}

// get renderer.
func (e *Engine) get(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Get session.
	session, err := e.sessionStore.Get(r)
	if err != nil {
		if r.URL.Query().Get("live-repair") != "" {
			e.Handler.ErrorHandler(ctx, fmt.Errorf("session corrupted: %w", err))
			return
		} else {
			log.Println(fmt.Errorf("session corrupted trying to repair: %w", err))
			e.sessionStore.Clear(w, r)
			q := r.URL.Query()
			q.Set("live-repair", "1")
			r.URL.RawQuery = q.Encode()
			http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
		}
		return
	}

	// Get socket.
	sock := NewSocket(session, e, false)

	// Run mount, this generates the state for the page we are on.
	data, err := e.Handler.MountHandler(ctx, sock)
	if err != nil {
		e.Handler.ErrorHandler(ctx, err)
		return
	}
	sock.Assign(data)

	// Handle any query parameters that are on the page.
	for _, ph := range e.Handler.paramsHandlers {
		data, err := ph(ctx, sock, NewParamsFromRequest(r))
		if err != nil {
			e.Handler.ErrorHandler(ctx, err)
			return
		}
		sock.Assign(data)
	}

	// Render the HTML to display the page.
	render, err := RenderSocket(ctx, e, sock)
	if err != nil {
		e.Handler.ErrorHandler(ctx, err)
		return
	}
	sock.UpdateRender(render)

	var rendered bytes.Buffer
	html.Render(&rendered, render)

	if err := e.sessionStore.Save(w, r, session); err != nil {
		e.Handler.ErrorHandler(ctx, err)
		return
	}

	w.WriteHeader(200)
	io.Copy(w, &rendered)
}

// serveWS serve a websocket request to the handler.
func (e *Engine) serveWS(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Get the session from the http request.
	session, err := e.sessionStore.Get(r)
	if err != nil {
		e.Handler.ErrorHandler(ctx, err)
		return
	}

	// https://github.com/nhooyr/websocket/issues/218
	// https://github.com/gorilla/websocket/issues/731
	if strings.Contains(r.UserAgent(), "Safari") {
		if e.acceptOptions == nil {
			e.acceptOptions = &websocket.AcceptOptions{}
		}
		e.acceptOptions.CompressionMode = websocket.CompressionDisabled
	}

	c, err := websocket.Accept(w, r, e.acceptOptions)
	if err != nil {
		e.Handler.ErrorHandler(ctx, err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "")
	writeTimeout(ctx, time.Second*5, c, Event{T: EventConnect})
	{
		err := e._serveWS(ctx, r, session, c)
		if errors.Is(err, context.Canceled) {
			return
		}
		switch websocket.CloseStatus(err) {
		case websocket.StatusNormalClosure:
			return
		case websocket.StatusGoingAway:
			return
		default:
			log.Println(fmt.Errorf("ws closed with status (%d): %w", websocket.CloseStatus(err), err))
			return
		}
	}
}

// _serveWS implement the logic for a web socket connection.
func (e *Engine) _serveWS(ctx context.Context, r *http.Request, session Session, c *websocket.Conn) error {
	// Get the sessions socket and register it with the server.
	sock := NewSocket(session, e, true)
	sock.assignWS(c)
	e.AddSocket(sock)
	defer e.DeleteSocket(sock)

	// Internal errors.
	internalErrors := make(chan error)

	// Event errors.
	eventErrors := make(chan ErrorEvent)

	// Handle events coming from the websocket connection.
	go func() {
		for {
			t, d, err := c.Read(ctx)
			if err != nil {
				internalErrors <- err
				break
			}
			switch t {
			case websocket.MessageText:
				var m Event
				if err := json.Unmarshal(d, &m); err != nil {
					internalErrors <- err
					break
				}
				switch m.T {
				case EventParams:
					if err := e.CallParams(ctx, sock, m); err != nil {
						switch {
						case errors.Is(err, ErrNoEventHandler):
							log.Println("event error", m, err)
						default:
							eventErrors <- ErrorEvent{Source: m, Err: err.Error()}
						}
					}
				default:
					if err := e.CallEvent(ctx, m.T, sock, m); err != nil {
						switch {
						case errors.Is(err, ErrNoEventHandler):
							log.Println("event error", m, err)
						default:
							eventErrors <- ErrorEvent{Source: m, Err: err.Error()}
						}
					}
				}
				render, err := RenderSocket(ctx, e, sock)
				if err != nil {
					internalErrors <- fmt.Errorf("socket handle error: %w", err)
				} else {
					sock.UpdateRender(render)
				}
				if err := sock.Send(EventAck, nil, WithID(m.ID)); err != nil {
					internalErrors <- fmt.Errorf("socket send error: %w", err)
				}
			case websocket.MessageBinary:
				log.Println("binary messages unhandled")
			}
		}
		close(internalErrors)
		close(eventErrors)
	}()

	// Run mount again now that eh socket is connected, passing true indicating
	// a connection has been made.
	data, err := e.Handler.MountHandler(ctx, sock)
	if err != nil {
		return fmt.Errorf("socket mount error: %w", err)
	}
	sock.Assign(data)

	// Run params again now that the socket is connected.
	for _, ph := range e.Handler.paramsHandlers {
		data, err := ph(ctx, sock, NewParamsFromRequest(r))
		if err != nil {
			return fmt.Errorf("socket params error: %w", err)
		}
		sock.Assign(data)
	}

	// Run render now that we are connected for the first time and we have just
	// mounted again. This will generate and send any patches if there have
	// been changes.
	render, err := RenderSocket(ctx, e, sock)
	if err != nil {
		return fmt.Errorf("socket render error: %w", err)
	}
	sock.UpdateRender(render)

	// Send events to the websocket connection.
	for {
		select {
		case msg := <-sock.msgs:
			if err := writeTimeout(ctx, time.Second*5, c, msg); err != nil {
				return fmt.Errorf("writing to socket error: %w", err)
			}
		case ee := <-eventErrors:
			d, err := json.Marshal(ee)
			if err != nil {
				return fmt.Errorf("writing to socket error: %w", err)
			}
			if err := writeTimeout(ctx, time.Second*5, c, Event{T: EventError, Data: d}); err != nil {
				return fmt.Errorf("writing to socket error: %w", err)
			}
		case err := <-internalErrors:
			if err != nil {
				d, err := json.Marshal(err.Error())
				if err != nil {
					return fmt.Errorf("writing to socket error: %w", err)
				}
				if err := writeTimeout(ctx, time.Second*5, c, Event{T: EventError, Data: d}); err != nil {
					return fmt.Errorf("writing to socket error: %w", err)
				}
				// Something catastrophic has happened.
				return fmt.Errorf("internal error: %w", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

//type HttpSocket struct {
//	*Socket
//}
//
//// NewHttpSocket creates a new http socket.
//func NewHttpSocket(s Session, e *Engine, connected bool) *HttpSocket {
//	return &HttpSocket{
//		Socket: NewSocket(s, e, connected),
//	}
//}

func httpContext(w http.ResponseWriter, r *http.Request) context.Context {
	ctx := r.Context()
	ctx = contextWithRequest(ctx, r)
	ctx = contextWithWriter(ctx, w)
	return ctx
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg Event) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(&msg)
	if err != nil {
		return fmt.Errorf("failed writeTimeout: %w", err)
	}

	return c.Write(ctx, websocket.MessageText, data)
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

// Clear a session.
func (c CookieStore) Clear(w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, &http.Cookie{
		Name:     c.sessionName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})
	return nil
}
