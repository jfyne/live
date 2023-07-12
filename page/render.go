package page

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
)

// HTML render some html with added template functions to support components. This
// passes the component state to be rendered.
//
// Template functions
// - "Event" takes an event string and scopes it for the component.
func HTML(layout string, c ComponentRender) RenderFunc {
	t := template.Must(template.New("").Funcs(templateFuncs(c)).Parse(layout))
	return Template(t, c)
}

// Template render a go template.
func Template(t *template.Template, c ComponentRender) RenderFunc {
	return func(w io.Writer) error {
		if err := t.Execute(w, c); err != nil {
			return err
		}
		return nil
	}
}

// Templte create a RenderFunc from an fs.FS
func FS(c ComponentRender, f fs.FS, file string) RenderFunc {
	t, err := ParseFS(c, f, file)
	if err != nil {
		panic(err)
	}
	return func(w io.Writer) error {
		if err := t.Execute(w, c); err != nil {
			return err
		}
		return nil
	}
}

// ParseFS parse a file system and inject our template funcs.
func ParseFS(c ComponentRender, f fs.FS, file string) (*template.Template, error) {
	return template.New(file).Funcs(templateFuncs(c)).ParseFS(f, file)
}

func templateFuncs(c ComponentRender) template.FuncMap {
	return template.FuncMap{
		"Event": c.Event,
		"Component": func(c ComponentRender) template.HTML {
			var buf bytes.Buffer
			if err := c.Render().Render(&buf); err != nil {
				return template.HTML(fmt.Sprintf("error rendering: %s", err))
			}
			return template.HTML(buf.String())
		},
	}
}

// RenderFunc a helper function to ease the rendering of nodes.
type RenderFunc func(io.Writer) error

// Render take a writer and render the func.
func (r RenderFunc) Render(w io.Writer) error {
	return r(w)
}
