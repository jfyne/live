package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/jfyne/live"
)

const (
	nextPage = "next-page"
)

type list struct {
	Items []string
	Page  int
}

func newList(s *live.Socket) *list {
	l, ok := s.Assigns().(*list)
	if !ok {
		l = &list{
			Items: []string{},
			Page:  0,
		}
	}
	return l
}

func (l list) NextPage() int {
	return l.Page + 1
}

func main() {
	t, err := template.ParseFiles("root.html", "pagination/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h := live.NewHandler(live.WithTemplateRenderer(t))

	// Set the mount function for this handler.
	h.MountHandler = func(ctx context.Context, s *live.Socket) (any, error) {
		return newList(s), nil
	}

	// Set the handle params function. This gets called after mount and contains the URL
	// query string values in the params map. This will also get called whenever the query
	// string is changed on the page.
	h.HandleParams(func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
		l := newList(s)
		l.Page = p.Int("page")
		l.Items = getPageOfItems(l.Page)
		return l, nil
	})

	// Alternative method to get to next page, using the server side Patch event.
	h.HandleEvent(nextPage, func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
		page := p.Int("page")
		v := url.Values{}
		v.Add("page", fmt.Sprintf("%d", page))
		s.PatchURL(v)
		return s.Assigns(), nil
	})

	// Run the server.
	http.Handle("/", live.NewHttpHandler(context.Background(), h))
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

// getPageOfItems in real life would be a service or a database call to get a
// page.
func getPageOfItems(page int) []string {
	start := page * itemsPerPage
	end := start + itemsPerPage
	if start >= len(items) || end > len(items) {
		return []string{}
	}
	return items[start:end]
}

const (
	itemCount    = 100
	itemsPerPage = 5
)

var items []string

func init() {
	for i := 0; i < itemCount; i++ {
		items = append(items, fmt.Sprintf("This is item %d", i))
	}
}
