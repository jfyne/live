package page

import (
	"html/template"
	"io"
)

// HTML render some html with added template functions to support components. This
// passes the component state to be rendered.
//
// Template functions
// - "Event" takes an event string and scopes it for the component.
func HTML(layout string, c ComponentRender) RenderFunc {
	t := template.Must(template.New("").Funcs(templateFuncs(c)).Parse(layout))
	return func(w io.Writer) error {
		if err := t.Execute(w, c); err != nil {
			return err
		}
		return nil
	}
}

func templateFuncs(c ComponentRender) template.FuncMap {
	return template.FuncMap{
		"Event": c.Event,
	}
}

// RenderFunc a helper function to ease the rendering of nodes.
type RenderFunc func(io.Writer) error

// Render take a writer and render the func.
func (r RenderFunc) Render(w io.Writer) error {
	return r(w)
}
