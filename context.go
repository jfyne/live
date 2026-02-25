package live

import (
	"context"
	"net/http"
)

type contextKey string

const (
	requestKey          contextKey = "context_request"
	writerKey           contextKey = "context_writer"
	sessionIDCtxKey     contextKey = "context_session_id"
	islandIDCtxKey      contextKey = "context_island_id"
	engineCtxKey        contextKey = "context_engine"
	selfEventQueueCtxKey contextKey = "context_self_event_queue"
)

// contextWithRequest embed the initiating request within the context.
func contextWithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, requestKey, r)
}

// Request extracts the original HTTP request from a context.
// This is useful in island handlers to access request headers, cookies, or other
// HTTP metadata.
//
// Returns nil if no request is stored in the context.
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

// Writer extracts the HTTP response writer from a context.
// This is useful in island handlers to write headers or perform HTTP-specific operations.
//
// Returns nil if no writer is stored in the context.
func Writer(ctx context.Context) http.ResponseWriter {
	data := ctx.Value(writerKey)
	w, ok := data.(http.ResponseWriter)
	if !ok {
		return nil
	}
	return w
}

// contextWithSessionID embeds the session ID within the context.
func contextWithSessionID(ctx context.Context, id SessionID) context.Context {
	return context.WithValue(ctx, sessionIDCtxKey, id)
}

// sessionIDFromContext extracts the session ID from a context.
// Returns an empty SessionID if no session ID is stored in the context.
func sessionIDFromContext(ctx context.Context) SessionID {
	data := ctx.Value(sessionIDCtxKey)
	id, ok := data.(SessionID)
	if !ok {
		return ""
	}
	return id
}

// contextWithIslandID embeds the island ID within the context.
func contextWithIslandID(ctx context.Context, id IslandID) context.Context {
	return context.WithValue(ctx, islandIDCtxKey, id)
}

// islandIDFromContext extracts the island ID from a context.
// Returns an empty IslandID if no island ID is stored in the context.
func islandIDFromContext(ctx context.Context) IslandID {
	data := ctx.Value(islandIDCtxKey)
	id, ok := data.(IslandID)
	if !ok {
		return ""
	}
	return id
}

// contextWithEngine embeds the IslandEngine within the context.
func contextWithEngine(ctx context.Context, engine *IslandEngine) context.Context {
	return context.WithValue(ctx, engineCtxKey, engine)
}

// engineFromContext extracts the IslandEngine from a context.
// Returns nil if no engine is stored in the context.
func engineFromContext(ctx context.Context) *IslandEngine {
	data := ctx.Value(engineCtxKey)
	engine, ok := data.(*IslandEngine)
	if !ok {
		return nil
	}
	return engine
}

// contextWithSelfEventQueue embeds the self-event queue within the context.
func contextWithSelfEventQueue(ctx context.Context, queue *[]Event) context.Context {
	return context.WithValue(ctx, selfEventQueueCtxKey, queue)
}

// selfEventQueueFromContext extracts the self-event queue from a context.
// Returns nil if no queue is stored in the context.
func selfEventQueueFromContext(ctx context.Context) *[]Event {
	data := ctx.Value(selfEventQueueCtxKey)
	queue, ok := data.(*[]Event)
	if !ok {
		return nil
	}
	return queue
}
