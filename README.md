# ⚡ live

Real-time user experiences with server-rendered HTML in Go. Inspired by and
borrowing from Phoenix LiveViews.

Live is intended as a replacement for React, Vue, Angular etc. You can write
an interactive web app just using Go and its templates.

![](https://github.com/jfyne/live-examples/blob/main/chat.gif)

The structures provided in this package are compatible with `net/http`, so will play
nicely with middleware and other frameworks.

## Other implementations

- [Fiber](https://github.com/jfyne/live-contrib/tree/main/livefiber) a live handler for [Fiber](https://github.com/gofiber/fiber).

## Community

For bugs please use github issues. If you have a question about design or adding features, I
am happy to chat about it in the discussions tab.

Discord server is [here](https://discord.gg/TuMNaXJMUG).

## Getting Started

### Install

```
go get github.com/jfyne/live
```

See the [examples](https://github.com/jfyne/live-examples) for usage.

### First handler

Here is an example demonstrating how we would make a simple thermostat. Live is compatible
with `net/http`.

[embedmd]:# (example_test.go)
```go
package live

import (
	"bytes"
	"context"
	"html/template"
	"io"
	"net/http"
)

// Model of our thermostat.
type ThermoModel struct {
	C float32
}

// Helper function to get the model from the socket data.
func NewThermoModel(s Socket) *ThermoModel {
	m, ok := s.Assigns().(*ThermoModel)
	// If we haven't already initialised set up.
	if !ok {
		m = &ThermoModel{
			C: 19.5,
		}
	}
	return m
}

// thermoMount initialises the thermostat state. Data returned in the mount function will
// automatically be assigned to the socket.
func thermoMount(ctx context.Context, s Socket) (interface{}, error) {
	return NewThermoModel(s), nil
}

// tempUp on the temp up event, increase the thermostat temperature by .1 C. An EventHandler function
// is called with the original request context of the socket, the socket itself containing the current
// state and and params that came from the event. Params contain query string parameters and any
// `live-value-` bindings.
func tempUp(ctx context.Context, s Socket, p Params) (interface{}, error) {
	model := NewThermoModel(s)
	model.C += 0.1
	return model, nil
}

// tempDown on the temp down event, decrease the thermostat temperature by .1 C.
func tempDown(ctx context.Context, s Socket, p Params) (interface{}, error) {
	model := NewThermoModel(s)
	model.C -= 0.1
	return model, nil
}

// Example shows a simple temperature control using the
// "live-click" event.
func Example() {

	// Setup the handler.
	h := NewHandler()

	// Mount function is called on initial HTTP load and then initial web
	// socket connection. This should be used to create the initial state,
	// the socket Connected func will be true if the mount call is on a web
	// socket connection.
	h.HandleMount(thermoMount)

	// Provide a render function. Here we are doing it manually, but there is a
	// provided WithTemplateRenderer which can be used to work with `html/template`
	h.HandleRender(func(ctx context.Context, data *RenderContext) (io.Reader, error) {
		tmpl, err := template.New("thermo").Parse(`
            <div>{{.Assigns.C}}</div>
            <button live-click="temp-up">+</button>
            <button live-click="temp-down">-</button>
            <!-- Include to make live work -->
            <script src="/live.js"></script>
        `)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, err
		}
		return &buf, nil
	})

	// This handles the `live-click="temp-up"` button. First we load the model from
	// the socket, increment the temperature, and then return the new state of the
	// model. Live will now calculate the diff between the last time it rendered and now,
	// produce a set of diffs and push them to the browser to update.
	h.HandleEvent("temp-up", tempUp)

	// This handles the `live-click="temp-down"` button.
	h.HandleEvent("temp-down", tempDown)

	http.Handle("/thermostat", NewHttpHandler(NewCookieStore("session-name", []byte("weak-secret")), h))

	// This serves the JS needed to make live work.
	http.Handle("/live.js", Javascript{})

	http.ListenAndServe(":8080", nil)
}
```

Notice the `script` tag. Live's javascript is embedded within the library for ease of use, and
is required to be included for it to work. You can also use the companion
[npm package](https://www.npmjs.com/package/@jfyne/live) to add to any existing web app build
pipeline.

### Live components

Live can also render components. These are an easy way to encapsulate event logic and make it repeatable across a page.
The [components examples](https://github.com/jfyne/live-examples/tree/main/components) show how to create
components. Those are then used in the [world clocks example](https://github.com/jfyne/live-examples/tree/main/clocks).

[embedmd]:# (page/example_test.go)
```go
package page

import (
	"context"
	"io"
	"net/http"

	"github.com/jfyne/live"
)

// NewGreeter creates a new greeter component.
func NewGreeter(ID string, h live.Handler, s live.Socket, name string) (*Component, error) {
	return NewComponent(
		ID,
		h,
		s,
		WithMount(func(ctx context.Context, c *Component) error {
			c.State = name
			return nil
		}),
		WithRender(func(w io.Writer, c *Component) error {
			// Render the greeter, here we are including the script just to make this toy example work.
			return HTML(`
                <div class="greeter">Hello {{.}}</div>
                <script src="/live.js"></script>
            `, c).Render(w)
		}),
	)
}

func Example() {
	h := live.NewHandler(
		WithComponentMount(func(ctx context.Context, h live.Handler, s live.Socket) (*Component, error) {
			return NewGreeter("hello-id", h, s, "World!")
		}),
		WithComponentRenderer(),
	)

	http.Handle("/", live.NewHttpHandler(live.NewCookieStore("session-name", []byte("weak-secret")), h))
	http.Handle("/live.js", live.Javascript{})
	http.ListenAndServe(":8080", nil)
}
```

## Navigation

Live provides functionality to use the browsers pushState API to update its query parameters. This can be done from
both the client side and the server side.

### Client side

The `live-patch` handler should be placed on an `a` tag element as it reads the `href` attribute in order to apply
the URL patch.

```html
<a live-patch href="?page=2">Next page</a>
```

Clicking on this tag will result in the browser URL being updated, and then an event sent to the backend which will
trigger the handler's `HandleParams` callback. With the query string being available in the params map of the handler.

```go
h.HandleParams(func(s *live.Socket, p live.Params) (interface{}, error) {
    ...
    page := p.Int("page")
    ...
})
```

### Server side

Using the Socket's `PatchURL` func the serverside can make the client update the browsers URL, which will then trigger the `HandleParams` func.

### Redirect

The server can also trigger a redirect if the Socket's `Redirect` func is called. This will simulate an HTTP redirect
using `window.location.replace`.

## Features

### Click Events

- [ ] live-capture-click
- [x] live-click
- [x] live-value-*

The `live-click` binding is used to send click events to the server.

```html
<div live-click="inc" live-value-myvar1="val1" live-value-myvar2="val2"></div>
```

See the [buttons example](https://github.com/jfyne/live-examples/tree/main/buttons) for usage.

### Focus / Blur Events

- [x] live-window-focus
- [x] live-window-blur
- [x] live-focus
- [x] live-blur

Focus and blur events may be bound to DOM elements that emit such events,
using the `live-blur`, and `live-focus` bindings, for example:

```html
<input name="email" live-focus="myfocus" live-blur="myblur"/>
```

### Key Events

- [x] live-window-keyup
- [x] live-window-keydown
- [x] live-keyup
- [x] live-keydown
- [x] live-key

The onkeydown, and onkeyup events are supported via the `live-keydown`, and `live-keyup`
bindings. Each binding supports a `live-key` attribute, which triggers the event for the
specific key press. If no `live-key` is provided, the event is triggered for any key press.
When pushed, the value sent to the server will contain the "key" that was pressed.

See the [buttons example](https://github.com/jfyne/live-examples/tree/main/buttons) for usage.

### Form Events

- [ ] live-auto-recover
- [ ] live-trigger-action
- [ ] live-disable-with
- [ ] live-feedback-for
- [x] live-submit
- [x] live-change

To handle form changes and submissions, use the `live-change` and `live-submit` events. In general,
it is preferred to handle input changes at the form level, where all form fields are passed to the
handler's event handler given any single input change. For example, to handle real-time form validation
and saving, your template would use both `live-change` and `live-submit` bindings.

See the [form example](https://github.com/jfyne/live-examples/tree/main/todo) for usage.

### Rate Limiting

- [x] live-debounce
- [ ] live-throttle

All events can be rate-limited on the client by using the `live-debounce` and `live-throttle` bindings,
with the following behavior:

`live-debounce` accepts either an integer timeout value (in milliseconds),
or "blur". When an integer is provided, emitting the event is delayed by the specified milliseconds. When
"blur" is provided, emitting the event is delayed until the field is blurred by the user. Debouncing is typically
used for input elements.

`live-throttle` accepts an integer timeout value to throttle the event in milliseconds. Unlike debounce,
throttle will immediately emit the event, then rate limit it at once per provided timeout. Throttling is
typically used to rate limit clicks, mouse and keyboard actions.

### Dom Patching

- [x] live-update

A container can be marked with `live-update`, allowing the DOM patch operations
to avoid updating or removing portions of the view, or to append or prepend the
updates rather than replacing the existing contents. This is useful for client-side
interop with existing libraries that do their own DOM operations. The following
`live-update` values are supported:

- `replace` - replaces the element with the contents
- `ignore` - ignores updates to the DOM regardless of new content changes
- `append` - append the new DOM contents instead of replacing
- `prepend` - prepend the new DOM contents instead of replacing

When using `live-update` If using "append" or "prepend", a DOM ID must be set
for each child.

See the [chat example](https://github.com/jfyne/live-examples/tree/main/chat) for usage.

### JS Interop

- [x] live-hook

### Hooks

Hooks take the following form. They allow additional javscript to be during a
page lifecycle.

[embedmd]:# (web/src/interop.ts)
```ts
/**
 * Hooks supplied for interop.
 */
export interface Hooks {
    [id: string]: Hook;
}

/**
 * A hook for running external JS.
 */
export interface Hook {
    /**
     * The element has been added to the DOM and its server
     * LiveHandler has finished mounting
     */
    mounted?: () => void;

    /**
     * The element is about to be updated in the DOM.
     * Note: any call here must be synchronous as the operation
     * cannot be deferred or cancelled.
     */
    beforeUpdate?: () => void;

    /**
     * The element has been updated in the DOM by the server
     */
    updated?: () => void;

    /**
     * The element is about to be removed from the DOM.
     * Note: any call here must be synchronous as the operation
     * cannot be deferred or cancelled.
     */
    beforeDestroy?: () => void;

    /**
     * The element has been removed from the page, either by
     * a parent update, or by the parent being removed entirely
     */
    destroyed?: () => void;

    /**
     * The element's parent LiveHandler has disconnected from
     * the server
     */
    disconnected?: () => void;

    /**
     * The element's parent LiveHandler has reconnected to the
     * server
     */
    reconnected?: () => void;
}

/**
 * The DOM management interace. This allows external JS libraries to
 * interop with Live.
 */
export interface DOM {
    /**
     * The fromEl and toEl DOM nodes are passed to the function
     * just before the DOM patch operations occurs in Live. This
     * allows external libraries to (re)initialize DOM elements
     * or copy attributes as necessary as Live performs its own
     * patch operations. The update operation cannot be cancelled
     * or deferred, and the return value is ignored.
     */
    onBeforeElUpdated?: (fromEl: Element, toEl: Element) => void;
}
```

In scope when these functions are called:

- `el` - attribute referencing the bound DOM node,
- `pushEvent(event: { t: string, d: any })` - method to push an event from the client to the Live server
- `handleEvent(event: string, cb: ((payload: any) => void))` - method to handle an event pushed from the server.

See the [chat example](https://github.com/jfyne/live-examples/tree/main/chat) for usage.

### Integrating with your app

There are two ways to inegrate javascript into your applications. The first is the simplest, using the built
in javascript handler. This includes client side code to initialise the live handler and automatically looks for
hooks at `window.Hooks`. All of the examples use this method.

See the [chat example](https://github.com/jfyne/live-examples/tree/main/chat) for usage.

The second method is suited for more complex apps, there is a companion package published on npm. The version
should be kept in sync with the current go version.

```bash
> npm i @jfyne/live
```

This can then be used to initialise the live handler on a page

```typescript
import { Live } from '@jfyne/live';

const hooks = {};

const live = new Live(hooks);
live.init();
```

This allows more control over how hooks are passed to live, and when it should be initialised. It is expected
that you would then build your compiled javsacript and serve it. See the
[alpine example](https://github.com/jfyne/live-examples/tree/main/alpine).

## Errors and exceptions

There are two types of errors in a live handler, and how these are handled are separate.

### Unexpected errors

Errors that occur during the initial mount, initial render and web socket
upgrade process are handled by the handler `ErrorHandler` func.

Errors that occur while handling incoming web socket messages will trigger
a response back with the error.

### Expected errors

In general errors which you expect to happen such as form validations etc.
should be handled by just updating the data on the socket and
re-rendering.

If you return an error in the event handler live will send an `"err"` event
to the socket. You can handle this with a hook. An example of this can be
seen in the [error example](https://github.com/jfyne/live-examples/tree/main/error).

##  Loading state and errors

By default, the following classes are applied to the handlers body:

- `live-connected` - applied when the view has connected to the server
- `live-disconnected` - applied when the view is not connected to the server
- `live-error` - applied when an error occurs on the server. Note, this class will be applied in conjunction with `live-disconnected` if connection to the server is lost.

All `live-` event bindings apply their own css classes when pushed. For example the following markup:

```html
<button live-click="clicked" live-window-keydown="key">...</button>
```

On click, would receive the `live-click-loading` class, and on keydown would 
receive the `live-keydown-loading` class. The css loading classes are maintained
until an acknowledgement is received on the client for the pushed event.

The following events receive css loading classes:

- `live-click` - `live-click-loading`
- `live-change` - `live-change-loading`
- `live-submit` - `live-submit-loading`
- `live-focus` - `live-focus-loading`
- `live-blur` - `live-blur-loading`
- `live-window-keydown` - `live-keydown-loading`
- `live-window-keyup` - `live-keyup-loading`

## Broadcasting to different nodes

In production it is often required to have multiple instances of the same application running, in order to handle this
live has a PubSub element. This allows nodes to publish onto topics and receive those messages as if they were all
running as the same instance. See the [cluster example](https://github.com/jfyne/live-examples/tree/main/cluster) for
usage.

## Uploads

Live supports interactive file uploads with progress indication. See the [uploads example](https://github.com/jfyne/live-examples/tree/main/uploads)
for usage.

### Features

Accept specification - Define accepted file types, max number of entries, max file size, etc. When the client
selects file(s), the file metadata can be validated with a helper function.

Reactive entries - Uploads are populated in the `.Uploads` template context. Entries automatically respond
to progress and errors.

### Entry validataion

File selection triggers the usual form change event and there is a helper function to validate the uploads. 
Use `live.ValidateUploads` to validate the incoming files. Any validation errors will be available in the `.Uploads`
context in the template.

### Consume the uploads

When a form is submitted files will first be uploaded to a staging area, then the submit event is triggered. Within the event
handler use the `live.ConsumeUploads` helper function to then move the uploaded files to where you need them.
