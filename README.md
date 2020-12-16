# live

An attempt to bring something similar to phoenix live views to golang.

## Getting Started

### Install

```
go get github.com/jfyne/live
```

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

We would then define a view like this (from the clock example):

```html
{{ define "title" }} {{.FormattedTime}} {{ end }}
{{ define "view" }}
<time>{{.FormattedTime}}</time>
{{ end }}
```

And in go

```go
	view, err := live.NewView("/clock", []string{"examples/root.html", "examples/clock/view.html"})
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

- [ ] live-hook

## TODO

- [ ] Golang http middleware support
