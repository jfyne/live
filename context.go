package live

import (
	"context"
	"net/http"
)

type contextKey string

const (
	requestKey contextKey = "context_request"
	writerKey  contextKey = "context_writer"
)

// contextWithRequest embed the initiating request within the context.
func contextWithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, requestKey, r)
}

// Request pulls out an initiating request from a context.
func Request(ctx context.Context) *http.Request {
	data := ctx.Value(requestKey)
	r, ok := data.(*http.Request)
	if !ok {
		return nil
	}
	return r
}

// contextWithWriter embed the response writer within the context.
func contextWithWriter(ctx context.Context, w http.ResponseWriter) context.Context {
	return context.WithValue(ctx, writerKey, w)
}

// Request pulls out an initiating request from a context.
func Writer(ctx context.Context) http.ResponseWriter {
	data := ctx.Value(writerKey)
	w, ok := data.(http.ResponseWriter)
	if !ok {
		return nil
	}
	return w
}
