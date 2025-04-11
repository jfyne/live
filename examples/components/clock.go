package components

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jfyne/live"
	"github.com/jfyne/live/page"
)

const (
	tick = "tick"
)

// ClockState the state we are tracking per clock.
type ClockState struct {
	TZ   string
	Time time.Time
	loc  *time.Location
}

// FormattedTime output the time in a nice format.
func (c ClockState) FormattedTime() string {
	return c.Time.Format("15:04:05")
}

// Update the states time.
func (c *ClockState) Update(t time.Time) {
	c.Time = t.In(c.loc)
}

// NewClockState create a new clock state from a timezone string.
func NewClockState(timezone string) (*ClockState, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(location)
	c := &ClockState{
		Time: now,
		loc:  location,
		TZ:   timezone,
	}
	return c, nil
}

// clockRegister register the clocks events.
func clockRegister(c *page.Component) error {
	// The clock listens for a tick event, then sends a new one after a second. On this
	// event it updates its own time.
	c.HandleSelf(tick, func(ctx context.Context, d any) (any, error) {
		clock, ok := c.State.(*ClockState)
		if !ok {
			return nil, fmt.Errorf("no clock data")
		}
		clock.Update(d.(time.Time))

		go func(sock *live.Socket) {
			time.Sleep(1 * time.Second)
			c.Self(ctx, sock, tick, time.Now())
		}(c.Socket)

		return clock, nil
	})
	return nil
}

// clockMount initialise the clock component.
func clockMount(timezone string) page.MountHandler {
	return func(ctx context.Context, c *page.Component) error {
		// If we are mounting on connection send the first tick event.
		if c.Socket.Connected() {
			go func() {
				time.Sleep(1 * time.Second)
				c.Self(ctx, c.Socket, tick, time.Now())
			}()
		}
		state, err := NewClockState(timezone)
		if err != nil {
			return err
		}
		c.State = state
		return nil
	}
}

// clockRender render the clock component.
func clockRender(w io.Writer, c *page.Component) error {
	// The page.HTML helper function renders a go template and passes in the
	// component state.
	return page.HTML(`
        <div>
            <p>{{.TZ}}</p>
            <time>{{.FormattedTime}}</time>
        </div>
    `, c).Render(w)
}

// NewClock create a new clock component.
func NewClock(ID string, h *live.Handler, s *live.Socket, timezone string) (*page.Component, error) {
	return page.NewComponent(ID, h, s,
		page.WithRegister(clockRegister),
		page.WithMount(clockMount(timezone)),
		page.WithRender(clockRender),
	)
}
