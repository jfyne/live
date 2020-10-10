package live

import "errors"

// ErrViewMisconfigured returned when a view is not configured
// correctly.
var ErrViewMisconfigured = errors.New("view misconfigured")

// ErrNoEventHandler returned when a view has no event handler for that event.
var ErrNoEventHandler = errors.New("view missing event handler")
