# live

An attempt to bring something similar to phoenix live views to golang.

Compatible with `net/http`, so will play nicely with middleware etc.

## Getting Started

### Install

```
go get github.com/jfyne/live
```

See the [examples](https://github.com/jfyne/live/blob/master/examples) for usage.

### First view

As of writing, each view expects there to be a `root.html` template which it will render
into. See any of the [examples](https://github.com/jfyne/live/blob/master/examples) to see 
this in action.

```html
<!doctype html>
<html>
    <head>
        <title>{{ template "title" . }}</title>
    </head>
    <body>
        {{ template "view" . }}
        <!-- This is embedded in the binary and enables live to work -->
        <script type="text/javascript" src="/live.js"></script>
    </body>
</html>
```

Notice the `script` tag. Live's javascript is embedded within its binary for ease of use, and
is required to be included for it to work.

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
view, _ := live.NewView(t, "session-key", sessionStore)
```

And then just serve like you normallly would

```go
// Here we are using `http.Handle` but you could use
// `gorilla/mux` or whatever you want. 

// Serve the view itself.
http.Handle("/clock", view)

// This serves the javscript for live to work and is required. This is what
// we referenced in the `root.html`.
http.Handle("/live.js", live.Javascript{})

http.ListenAndServe(":8080", nil)
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

See the [buttons example](https://github.com/jfyne/live/blob/master/examples/buttons) for usage.

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

See the [buttons example](https://github.com/jfyne/live/blob/master/examples/buttons) for usage.

### Form Events

- [ ] live-auto-recover
- [ ] live-trigger-action
- [ ] live-disable-with
- [ ] live-feedback-for
- [x] live-submit
- [x] live-change

To handle form changes and submissions, use the `live-change` and `live-submit` events. In general,
it is preferred to handle input changes at the form level, where all form fields are passed to the
views event handler given any single input change. For example, to handle real-time form validation
and saving, your template would use both `live-change` and `live-submit` bindings.

See the [form example](https://github.com/jfyne/live/blob/master/examples/form) for usage.

### Rate Limiting

- [ ] live-throttle
- [ ] live-debounce

### Dom Patching

- [ ] live-update

### JS Interop

- [x] live-hook

Hooks take the following form.

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
     * LiveView has finished mounting
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
     * The element's parent LiveView has disconnected from
     * the server
     */
    disconnected?: () => void;

    /**
     * The element's parent LiveView has reconnected to the
     * server
     */
    reconnected?: () => void;
}
```

In scope when these functions are called.

- `el` - attribute referencing the bound DOM node,
- `pushEvent(event: { t: string, d: any })` - method to push an event from the client to the Live server
- `handleEvent(event: string, cb: ((payload: any) => void))` - method to handle an event pushed from the server.

See the [chat example](https://github.com/jfyne/live/blob/master/examples/chat) for usage.
