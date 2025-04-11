package components

import (
	"context"
	"fmt"
	"io"

	"github.com/jfyne/live"
	"github.com/jfyne/live/page"
	g "github.com/maragudk/gomponents"
	c "github.com/maragudk/gomponents/components"
	h "github.com/maragudk/gomponents/html"
)

const (
	validateTZ = "validate-tz"
	addTime    = "add-time"
)

// PageState the state we are tracking for our page.
type PageState struct {
	Title           string
	ValidationError string
	Clocks          []*page.Component
}

// newPageState create a new page state.
func newPageState(title string) *PageState {
	return &PageState{
		Title:  title,
		Clocks: []*page.Component{},
	}
}

// pageRegister register the pages events.
func pageRegister(c *page.Component) error {
	// Handler for the timezone entry validation.
	c.HandleEvent(validateTZ, func(_ context.Context, p live.Params) (any, error) {
		// Get the current page component state.
		state, _ := c.State.(*PageState)

		// Get the tz coming from the form.
		tz := p.String("tz")

		// Try to make a new ClockState, this will return an error if the
		// timezone is not real.
		if _, err := NewClockState(tz); err != nil {
			state.ValidationError = fmt.Sprintf("Timezone %s does not exist", tz)
			return state, nil
		}

		// If there was no error loading the clock state reset the
		// validation error.
		state.ValidationError = ""

		return state, nil
	})

	// Handler for adding a timezone.
	c.HandleEvent(addTime, func(_ context.Context, p live.Params) (any, error) {
		// Get the current page component state.
		state, _ := c.State.(*PageState)

		// Get the timezone sent from the form input.
		tz := p.String("tz")
		if tz == "" {
			return state, nil
		}

		// Use the page.Init function to create a new clock, register it and mount it.
		clock, err := page.Init(context.Background(), func() (*page.Component, error) {
			// Each clock requires its own unique stable ID. Events for each clock can then find
			// their own component.
			return NewClock(fmt.Sprintf("clock-%d", len(state.Clocks)+1), c.Handler, c.Socket, tz)
		})
		if err != nil {
			return state, err
		}

		// Update the page state with the new clock.
		state.Clocks = append(state.Clocks, clock)

		// Return the state to have it persisted.
		return state, nil
	})

	return nil
}

// pageMount initialise the page component.
func pageMount(title string) page.MountHandler {
	return func(_ context.Context, c *page.Component) error {
		// Create a new page state.
		c.State = newPageState(title)
		return nil
	}
}

// pageRender render the page component.
func pageRender(w io.Writer, cmp *page.Component) error {
	state, ok := cmp.State.(*PageState)
	if !ok {
		return fmt.Errorf("could not get state")
	}

	// Here we use the gomponents library to do typed rendering.
	// https://github.com/maragudk/gomponents
	return c.HTML5(c.HTML5Props{
		Title:    state.Title,
		Language: "en",
		Head: []g.Node{
			h.StyleEl(h.Type("text/css"),
				g.Raw(`body {font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol"; }`),
			),
		},
		Body: []g.Node{
			h.H1(g.Text("World Clocks")),
			h.Form(
				h.ID("tz-form"),
				g.Attr("live-change", cmp.Event(validateTZ)), // c.Event scopes the events to this component.
				g.Attr("live-submit", cmp.Event(addTime)),
				h.Div(
					h.P(g.Text("Try Europe/London or America/New_York")),
					h.Input(h.Name("tz")),
					g.If(state.ValidationError != "", h.Span(g.Text(state.ValidationError))),
				),
				h.Input(h.Type("submit"), g.If(state.ValidationError != "", h.Disabled())),
			),
			h.Div(
				g.Group(g.Map(state.Clocks, func(c *page.Component) g.Node {
					return page.Render(c)
				})),
			),
			h.Script(h.Src("/live.js")),
		},
	}).Render(w)
}

// NewPage create a new page component.
func NewPage(ID string, h *live.Handler, s *live.Socket, title string) (*page.Component, error) {
	return page.NewComponent(ID, h, s,
		page.WithRegister(pageRegister),
		page.WithMount(pageMount(title)),
		page.WithRender(pageRender),
	)
}
