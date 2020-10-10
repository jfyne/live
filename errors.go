package live

import "errors"

// ErrViewMisconfigured returned when a view is not configured
// correctly.
var ErrViewMisconfigured = errors.New("view misconfigured")
