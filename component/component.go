package component

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"

	"github.com/jfyne/live"
)

var _ RegisterHandler = defaultRegister
var _ MountHandler = defaultMount
var _ RenderHandler = defaultRender

// RegisterHandler the first part of the component lifecycle, this is called during component creation
// and is used to register any events that the component handles.
type RegisterHandler func(c *Component) error

// MountHandler the components mount function called on first GET request and again when the socket connects.
type MountHandler func(ctx context.Context, c *Component, r *http.Request, connected bool) error

// RenderHandler ths component.
type RenderHandler func(w io.Writer, c *Component) error

// EventHandler for a component, only needs the params as the event is scoped to both the socket and then component
// iteslef. Returns any component state that needs updating.
type EventHandler func(params map[string]interface{}) (interface{}, error)

// ComponentConstructor a func for creating a new component.
type ComponentConstructor func(ctx context.Context, h *live.Handler, r *http.Request, s *live.Socket) (Component, error)

// Component a self contained component on the page.
type Component struct {
	// ID identifies the component on the page.
	ID string

	// Handler a reference to the host handler.
	Handler *live.Handler

	// Socket a reference to the socket that this component
	// is scoped too.
	Socket *live.Socket

	// Register the
	Register RegisterHandler
	Mount    MountHandler
	Render   RenderHandler
	// State
	State interface{}
}

// New creates a new component and returns it. It does not register it or mount it.
func New(ID string, h *live.Handler, s *live.Socket, configurations ...ComponentConfig) (Component, error) {
	c := Component{
		ID:       ID,
		Handler:  h,
		Socket:   s,
		Register: defaultRegister,
		Mount:    defaultMount,
		Render:   defaultRender,
	}
	for _, conf := range configurations {
		if err := conf(&c); err != nil {
			return Component{}, err
		}
	}

	return c, nil
}

// Insit takes a ComponentConstructor and then registers and mounts the component.
func Init(ctx context.Context, construct func() (Component, error)) (Component, error) {
	comp, err := construct()
	if err != nil {
		return Component{}, fmt.Errorf("could not install component on construct: %w", err)
	}
	if err := comp.Register(&comp); err != nil {
		return Component{}, fmt.Errorf("could not install component on register: %w", err)
	}
	if err := comp.Mount(ctx, &comp, nil, true); err != nil {
		return Component{}, fmt.Errorf("could not install component on mount: %w", err)
	}
	return comp, nil
}

// Self sends an event scoped not only to this socket, but to this specific component instance. Or any
// components sharing the same ID.
func (c *Component) Self(s *live.Socket, event live.Event) {
	event.T = c.EventPrefix(event.T)
	c.Handler.Self(s, event)
}

// HandleSelf handles scoped incoming events send by a components Self function.
func (c *Component) HandleSelf(event string, handler EventHandler) {
	c.Handler.HandleSelf(c.EventPrefix(event), func(s *live.Socket, p map[string]interface{}) (interface{}, error) {
		state, err := handler(p)
		if err != nil {
			return s.Assigns(), err
		}
		c.State = state
		return s.Assigns(), nil
	})
}

// HandleEvent handles a component event sent from a connected socket.
func (c *Component) HandleEvent(event string, handler EventHandler) {
	c.Handler.HandleEvent(c.EventPrefix(event), func(s *live.Socket, p map[string]interface{}) (interface{}, error) {
		state, err := handler(p)
		if err != nil {
			return s.Assigns(), err
		}
		c.State = state
		return s.Assigns(), nil
	})
}

// EventPrefix is the prefix applied to an event in order to scope it correctly in the
// live.Handler lifecycle.
func (c *Component) EventPrefix(event string) string {
	return c.Socket.Session.ID + "--" + c.ID + "--" + event
}

func (c *Component) HTML(layout string) Node {
	t := template.Must(template.New("").Funcs(templateFuncs(c.ID, c.Socket.Session.ID)).Parse(layout))
	return NodeFunc(func(w io.Writer) error {
		if err := t.Execute(w, c.State); err != nil {
			return err
		}
		return nil
	})
}

func templateFuncs(componentID, socketID string) template.FuncMap {
	return template.FuncMap{
		"E": func(event string) string {
			return socketID + "--" + componentID + "--" + event
		},
	}
}

// defaultRegister is the default register handler which does nothing.
func defaultRegister(c *Component) error {
	return nil
}

// defaultMount is the default mount handler which does nothing.
func defaultMount(ctx context.Context, c *Component, r *http.Request, connected bool) error {
	return nil
}

// defaultRender is the default render handler which does nothing.
func defaultRender(w io.Writer, c *Component) error {
	_, err := w.Write([]byte(fmt.Sprintf("%+v", c.State)))
	return err
}

type Node interface {
	Render(w io.Writer) error
}

type NodeFunc func(io.Writer) error

func (n NodeFunc) Render(w io.Writer) error {
	return n(w)
}

func RenderComponent(c Component) NodeFunc {
	return NodeFunc(func(w io.Writer) error {
		return c.Render(w, &c)
	})
}
