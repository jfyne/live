package live

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	output := `<html _l00=""><head _l000=""></head><body _l001="" live-rendered="">test</body></html>`

	h := NewHandler()
	h.HandleRender(func(ctx context.Context, data *RenderContext) (io.Reader, error) {
		return strings.NewReader(output), nil
	})

	e := NewHttpHandler(NewTestStore("test"), h)

	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	ctx := httpContext(rr, req)
	e.get(ctx, rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
		return
	}
	if rr.Body.String() != output {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), output)
	}
}

func TestHandlerErrorNoRenderer(t *testing.T) {
	h := NewHandler()

	e := NewHttpHandler(NewTestStore("test"), h)

	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	ctx := httpContext(rr, req)
	e.get(ctx, rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusInternalServerError)
		return
	}
}
