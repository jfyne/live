package live

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
)

func TestContextWithRequest(t *testing.T) {
	t.Run("stores and retrieves request correctly", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com/test", nil)
		ctx := context.Background()

		ctx = contextWithRequest(ctx, req)
		retrievedReq := Request(ctx)
		if retrievedReq != req {
			t.Errorf("Expected request %v, got %v", req, retrievedReq)
		}
	})

	t.Run("returns nil for missing request", func(t *testing.T) {
		req := Request(context.Background())
		if req != nil {
			t.Errorf("Expected nil request, got %v", req)
		}
	})
}

func TestContextWithWriter(t *testing.T) {
	t.Run("stores and retrieves writer correctly", func(t *testing.T) {
		w := &testResponseWriter{}
		ctx := context.Background()

		ctx = contextWithWriter(ctx, w)
		retrievedWriter := Writer(ctx)
		if retrievedWriter != w {
			t.Errorf("Expected writer %v, got %v", w, retrievedWriter)
		}
	})

	t.Run("returns nil for missing writer", func(t *testing.T) {
		w := Writer(context.Background())
		if w != nil {
			t.Errorf("Expected nil writer, got %v", w)
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("context utilities are safe with concurrent access", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com/test", nil)
		w := &testResponseWriter{}

		ctx := context.Background()
		ctx = contextWithRequest(ctx, req)
		ctx = contextWithWriter(ctx, w)

		// Concurrent reads should be safe
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				retrievedReq := Request(ctx)
				retrievedWriter := Writer(ctx)
				if retrievedReq != req {
					t.Errorf("Expected request %v, got %v", req, retrievedReq)
				}
				if retrievedWriter != w {
					t.Errorf("Expected writer %v, got %v", w, retrievedWriter)
				}
			}()
		}
		wg.Wait()
	})
}

func TestTypeMismatch(t *testing.T) {
	t.Run("Request returns nil for type mismatch", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), requestKey, "not a request")
		req := Request(ctx)
		if req != nil {
			t.Errorf("Expected nil for type mismatch, got %v", req)
		}
	})

	t.Run("Writer returns nil for type mismatch", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), writerKey, "not a writer")
		w := Writer(ctx)
		if w != nil {
			t.Errorf("Expected nil for type mismatch, got %v", w)
		}
	})
}

func TestNilContextValues(t *testing.T) {
	t.Run("Request handles nil context value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), requestKey, nil)
		req := Request(ctx)
		if req != nil {
			t.Errorf("Expected nil for nil context value, got %v", req)
		}
	})

	t.Run("Writer handles nil context value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), writerKey, nil)
		w := Writer(ctx)
		if w != nil {
			t.Errorf("Expected nil for nil context value, got %v", w)
		}
	})
}

func TestMultipleGoroutines(t *testing.T) {
	t.Run("multiple goroutines can safely access same context", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com/test", nil)
		w := &testResponseWriter{}

		ctx := context.Background()
		ctx = contextWithRequest(ctx, req)
		ctx = contextWithWriter(ctx, w)

		const goroutines = 1000
		var wg sync.WaitGroup
		errors := make(chan error, goroutines)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Read both values
				retrievedReq := Request(ctx)
				retrievedWriter := Writer(ctx)

				if retrievedReq != req {
					errors <- fmt.Errorf("goroutine %d: expected request %v, got %v", id, req, retrievedReq)
					return
				}
				if retrievedWriter != w {
					errors <- fmt.Errorf("goroutine %d: expected writer %v, got %v", id, w, retrievedWriter)
					return
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		for err := range errors {
			t.Error(err)
		}
	})
}

// testResponseWriter is a minimal http.ResponseWriter implementation for testing
type testResponseWriter struct{}

func (w *testResponseWriter) Header() http.Header {
	return http.Header{}
}

func (w *testResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (w *testResponseWriter) WriteHeader(statusCode int) {}