# live

Real-time user experiences with server-rendered HTML in Go. Inspired by and
borrowing from Phoenix LiveViews.

Live is intended as a replacement for React, Vue, Angular etc. You can write
an interactive web app just using Go and its templates.

![](https://github.com/jfyne/live-examples/blob/main/chat.gif)

Compatible with `net/http`, so will play nicely with middleware and other frameworks.

## Roadmap

- Navigation
- Implement any missing phx events that make sense.
- File uploads.

## Getting Started

### Install

```
go get github.com/jfyne/live
```

See the [examples](https://github.com/jfyne/live-examples) for usage.

### First handler

Live can render any kind of template you want to give it, however we will start
with an `html/template` example.

```html
<!doctype html>
<html>
    <head>
        <title>{{ template "title" . }}</title>
    </head>
    <body>
        {{ template "view" . }}
        <!-- This is embedded in the library and enables live to work -->
        <script type="text/javascript" src="/live.js"></script>
    </body>
</html>
```

Notice the `script` tag. Live's javascript is embedded within the library for ease of use, and
is required to be included for it to work. You can also use the companion
[npm package](https://www.npmjs.com/package/@jfyne/live) to add to any existing web app build
pipeline.

We would then define a view like this (from the clock example):

```html
{{ define "title" }} {{.FormattedTime}} {{ end }}
{{ define "view" }}
<time>{{.FormattedTime}}</time>
{{ end }}
```

And in go

```go
t, _ := template.ParseFiles("examples/root.html", "examples/clock/view.html")
h, _ := live.NewHandler(sessionStore, live.WithTemplateRenderer(t))
```

And then just serve like you normallly would

```go
// Here we are using `http.Handle` but you could use
// `gorilla/mux` or whatever you want. 

// Serve the handler itself.
http.Handle("/clock", h)

// This serves the javscript for live to work. This is what
// we referenced in the `root.html`.
http.Handle("/live.js", live.Javascript{})

http.ListenAndServe(":8080", nil)
```

### Live components

Live can also render components. These are an easy way to encapsulate event logic and make it repeatable across a page.
The [components examples](https://github.com/jfyne/live-examples/tree/main/components) show how to create
components. Those are then used in the [world clocks example](https://github.com/jfyne/live-examples/tree/main/clocks).

```go
// NewGreeter creates a component that says hello to someone.
func NewGreeter(ID string, h *live.Handler, s *live.Socket, name string) (page.Component, error) {
    return page.NewComponent(
        ID,
        h,
        s,
        page.WithMount(func(ctx context.Context, c *page.Component, r *http.Request, connected bool) error {
            c.State = name
        }),
        page.WithRender(func(w io.Writer, c *page.Component) error {
            // Render the greeter, here we are including the script just to make this toy example work.
            return page.HTML(`
                <div class="greeter">Hello {{.}}</div>
                <script src="/live.js"></script>
            `, c).Render(w)
        }),
}

func main() {
    h, err := live.NewHandler(
        live.NewCookieStore("session-name", []byte("weak-secret")),
        page.WithComponentMount(func(ctx context.Context, h *live.Handler, r *http.Request, s *live.Socket) (page.Component, error) {
            return NewGreeter("hello-id", h, s, "World!")
        }),
        page.WithComponentRenderer(),
    )
    if err != nil {
        log.Fatal(err)
    }

    http.Handle("/", h)
    http.Handle("/live.js", live.Javascript{})
    http.ListenAndServe(":8080", nil)
}
```

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

- [ ] live-throttle
- [ ] live-debounce

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

```typescript
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
[npm example](https://github.com/jfyne/live-examples/tree/main/npm).

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
