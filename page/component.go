package page

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/jfyne/live"
)

// ComponentEventHandler for a component, only needs the params as the event is scoped to both the socket and then component
// itself. Returns any component state that needs updating.
type ComponentEventHandler func(ctx context.Context, p live.Params) (any, error)

// ComponentSelfHandler for a component, only needs the data as the event is scoped to both the socket and then component
// itself. Returns any component state that needs updating.
type ComponentSelfHandler func(ctx context.Context, data any) (any, error)

// ComponentMount describes the needed function for mounting a component.
type ComponentMount interface {
	Mount(context.Context) error
}

// ComponentRender descirbes the neded functions for rendering a component.
type ComponentRender interface {
	Render() RenderFunc
	Event(string) string
}

// Component describes all that is neded to describe a component.
type Component interface {
	isComponent
	componentInit
	componentRegister
	ComponentMount
	ComponentRender
}

type componentInit interface {
	init(ID string, h *live.Handler, s live.Socket)
}

type componentRegister interface {
	register(ID string, h *live.Handler, s live.Socket, comp any) error
}

type isComponent interface {
	_isComponent()
	_assignUploads(live.UploadContext)
}

// BaseComponent is a self contained component on the page. Components can be reused accross the application
// or used to compose complex interfaces by splitting events handlers and render logic into
// smaller pieces.
//
// Remember to use a unique ID and use the Event function which scopes the event-name
// to trigger the event in the right component.
type BaseComponent struct {
	// ID identifies the component on the page. This should be something stable, so that during the mount
	// it can be found again by the socket.
	// When reusing the same component this ID should be unique to avoid conflicts.
	ID string

	// Handler a reference to the host handler.
	Handler *live.Handler

	// Socket a reference to the socket that this component
	// is scoped too.
	Socket live.Socket

	// Any uploads.
	Uploads live.UploadContext
}

func (c BaseComponent) String() string {
	buf := &bytes.Buffer{}
	c.Render().Render(buf)
	return buf.String()
}

func (c BaseComponent) _isComponent() {}
func (c *BaseComponent) _assignUploads(uploads live.UploadContext) {
	c.Uploads = uploads
}

func (c *BaseComponent) init(ID string, h *live.Handler, s live.Socket) {
	c.ID = ID
	c.Handler = h
	c.Socket = s
}

// Mount a default component mount function.
func (c BaseComponent) Mount(ctx context.Context) error {
	return nil
}

// Render a default component render function.
func (c BaseComponent) Render() RenderFunc {
	return func(w io.Writer) error {
		return nil
	}
}

var compMethodDetect = regexp.MustCompile(`On([A-Za-z]*)`)
var compMethodSplit = regexp.MustCompile(`[A-Z][^A-Z]*`)

func (c *BaseComponent) register(ID string, h *live.Handler, s live.Socket, t any) error {
	c.ID = ID
	c.Handler = h
	c.Socket = s

	ty := reflect.TypeOf(t)
	va := reflect.ValueOf(t)
	for i := 0; i < va.NumMethod(); i++ {
		method := ty.Method(i)
		if !compMethodDetect.MatchString(method.Name) {
			continue
		}
		parts := compMethodSplit.FindAllString(method.Name, -1)
		if len(parts) < 2 {
			continue
		}
		c.handleEvent(eventName(parts), func(ctx context.Context, p live.Params) (any, error) {
			res := va.MethodByName(method.Name).Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(p)})
			switch len(res) {
			case 0:
				return t, nil
			case 1:
				err, ok := res[0].Interface().(error)
				if !ok {
					return t, nil
				}
				return t, err
			default:
				return t, nil
			}
		})
		c.handleSelf(eventName(parts), func(ctx context.Context, data any) (any, error) {
			res := va.MethodByName(method.Name).Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(data)})
			switch len(res) {
			case 0:
				return t, nil
			case 1:
				err, ok := res[0].Interface().(error)
				if !ok {
					return t, nil
				}
				return t, err
			default:
				return t, nil
			}
		})
	}
	return nil
}

func eventName(parts []string) string {
	out := []string{}
	for _, p := range parts[1:] {
		out = append(out, strings.ToLower(p))
	}
	return strings.Join(out, "-")
}

// Start begins the component's lifecycle.
func NewComponent(ctx context.Context, ID string, h *live.Handler, s live.Socket, comp Component) error {
	if err := comp.register(ID, h, s, comp); err != nil {
		return fmt.Errorf("could not spawn component on register: %w", err)
	}
	if err := comp.Mount(ctx); err != nil {
		return fmt.Errorf("could not spawn component on mount: %w", err)
	}
	return nil
}

// Self sends an event scoped not only to this socket, but to this specific component instance. Or any
// components sharing the same ID.
func (c *BaseComponent) Self(ctx context.Context, event string, data any) error {
	return c.Socket.Self(ctx, c.Event(event), data)
}

// HandleSelf handles scoped incoming events send by a components Self function.
func (c *BaseComponent) handleSelf(event string, handler ComponentSelfHandler) {
	c.Handler.HandleSelf(c.Event(event), func(ctx context.Context, s live.Socket, d any) (any, error) {
		_, err := handler(ctx, d)
		if err != nil {
			return s.Assigns(), err
		}
		//c.State = state
		return s.Assigns(), nil
	})
}

// HandleEvent handles a component event sent from a connected socket.
func (c *BaseComponent) handleEvent(event string, handler ComponentEventHandler) {
	c.Handler.HandleEvent(c.Event(event), func(ctx context.Context, s live.Socket, p live.Params) (any, error) {
		_, err := handler(ctx, p)
		if err != nil {
			return s.Assigns(), err
		}
		//c.State = state
		return s.Assigns(), nil
	})
}

// HandleParams handles parameter changes. Caution these handlers are not scoped to a specific component.
func (c *BaseComponent) handleParams(handler ComponentEventHandler) {
	c.Handler.HandleParams(func(ctx context.Context, s live.Socket, p live.Params) (any, error) {
		_, err := handler(ctx, p)
		if err != nil {
			return s.Assigns(), err
		}
		//c.State = state
		return s.Assigns(), nil
	})
}

// Event scopes an event string so that it applies to this instance of this component
// only.
func (c BaseComponent) Event(event string) string {
	return c.ID + "--" + event
}
