package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jfyne/live"
)

//go:embed *.html
var content embed.FS

const (
	// itemsPerPage is the number of items shown per page.
	itemsPerPage = 5
	// totalItems is the total number of items available.
	totalItems = 50
)

// PaginationState holds the state for a pagination island instance.
type PaginationState struct {
	Page       int
	Items      []string
	TotalPages int
}

// getItemsForPage returns the slice of item strings for the given page number.
// Items are named "Item N" (1-indexed).
func getItemsForPage(page int) []string {
	start := page * itemsPerPage
	if start >= totalItems {
		return []string{}
	}
	end := start + itemsPerPage
	if end > totalItems {
		end = totalItems
	}
	items := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		items = append(items, fmt.Sprintf("Item %d", i+1))
	}
	return items
}

// totalPagesCount returns the total number of pages.
func totalPagesCount() int {
	pages := totalItems / itemsPerPage
	if totalItems%itemsPerPage != 0 {
		pages++
	}
	return pages
}

// NewPaginationIsland creates a new pagination island definition.
func NewPaginationIsland() (*live.Island, error) {
	island, err := live.NewIsland(
		"pagination",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return &PaginationState{
				Page:       0,
				Items:      getItemsForPage(0),
				TotalPages: totalPagesCount(),
			}, nil
		}),
		live.WithHandleParams(func(ctx context.Context, state any, params live.Params) (any, error) {
			ps := state.(*PaginationState)
			page := params.Int("page")
			ps.Page = page
			ps.Items = getItemsForPage(page)
			return ps, nil
		}),
		live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
			state := rc.State.(*PaginationState)

			tmpl, err := template.ParseFS(content, "pagination.html")
			if err != nil {
				return nil, err
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, state); err != nil {
				return nil, err
			}

			return &buf, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	// next-page: advance to the next page and update the URL.
	err = island.HandleEvent("next-page", func(ctx context.Context, state any, params live.Params) (any, error) {
		ps := state.(*PaginationState)
		ps.Page++
		ps.Items = getItemsForPage(ps.Page)
		_ = live.PatchURL(ctx, map[string]string{"page": strconv.Itoa(ps.Page)})
		return ps, nil
	})
	if err != nil {
		return nil, err
	}

	// prev-page: go back to the previous page (minimum 0) and update the URL.
	err = island.HandleEvent("prev-page", func(ctx context.Context, state any, params live.Params) (any, error) {
		ps := state.(*PaginationState)
		if ps.Page > 0 {
			ps.Page--
		}
		ps.Items = getItemsForPage(ps.Page)
		_ = live.PatchURL(ctx, map[string]string{"page": strconv.Itoa(ps.Page)})
		return ps, nil
	})
	if err != nil {
		return nil, err
	}

	return island, nil
}

func main() {
	// Register the pagination island with the global registry.
	err := live.RegisterIsland("pagination", NewPaginationIsland)
	if err != nil {
		log.Fatal("Failed to register pagination island:", err)
	}

	// Create context for engine lifecycle.
	ctx := context.Background()

	// Create state store with 1-minute cleanup interval.
	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)

	// Create island engine.
	registry := live.DefaultRegistry()
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Set up WebSocket transport endpoint.
	wsConfig := live.DefaultTransportConfig()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		transport, err := live.UpgradeWebSocket(r.Context(), w, r, wsConfig)
		if err != nil {
			http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
			return
		}

		sessionID := live.SessionID(fmt.Sprintf("session-%d", time.Now().UnixNano()))
		session := live.NewSession(r.Context(), sessionID, transport)
		engine.AddSession(session)

		defer func() {
			engine.DeleteSession(sessionID)
		}()

		go func() {
			for event := range transport.Events() {
				log.Printf("Received event: type=%s, island=%s", event.T, event.Island)

				if event.T == "subscribe" && event.Island != "" {
					islandID := live.IslandID(event.Island)
					params, _ := event.Params()
					islandType := params.String("type")

					if islandType == "" {
						log.Printf("Warning: subscribe event for %s missing type, cannot mount", islandID)
						continue
					}

					_, err := engine.MountIsland(sessionID, islandID, islandType, live.Props{})
					if err != nil {
						log.Printf("Failed to mount island %s: %v", islandID, err)
					}
					continue
				}

				if err := engine.RouteEvent(sessionID, event); err != nil {
					log.Printf("Event routing error: %v", err)
				}
			}
		}()

		<-r.Context().Done()
	})

	// Set up SSE transport endpoints.
	sseConfig := live.DefaultTransportConfig()
	sseFactory := live.NewSSETransportFactory(sseConfig)

	http.HandleFunc("/live/sse", func(w http.ResponseWriter, r *http.Request) {
		transport, err := sseFactory.Upgrade(r.Context(), w, r)
		if err != nil {
			http.Error(w, "SSE upgrade failed", http.StatusBadRequest)
			return
		}

		sessionID := live.SessionID(live.GetSessionIDFromRequest(r))
		if sessionID == "" {
			sessionID = live.SessionID(fmt.Sprintf("session-%d", time.Now().UnixNano()))
		}
		session := live.NewSession(r.Context(), sessionID, transport)
		engine.AddSession(session)

		defer func() {
			engine.DeleteSession(sessionID)
		}()

		go func() {
			for event := range transport.Events() {
				log.Printf("SSE received event: type=%s, island=%s", event.T, event.Island)

				if event.T == "subscribe" && event.Island != "" {
					islandID := live.IslandID(event.Island)
					params, _ := event.Params()
					islandType := params.String("type")

					if islandType == "" {
						log.Printf("Warning: subscribe event for %s missing type, cannot mount", islandID)
						continue
					}

					_, err := engine.MountIsland(sessionID, islandID, islandType, live.Props{})
					if err != nil {
						log.Printf("Failed to mount island %s: %v", islandID, err)
					}
					continue
				}

				if err := engine.RouteEvent(sessionID, event); err != nil {
					log.Printf("Event routing error: %v", err)
				}
			}
		}()

		<-r.Context().Done()
	})

	// SSE POST handler for client-to-server events.
	http.HandleFunc("/live/post", func(w http.ResponseWriter, r *http.Request) {
		sseFactory.HandlePost(w, r)
	})

	// Serve the main HTML page.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		tmpl, err := template.ParseFS(content, "index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.Execute(w, nil); err != nil {
			slog.Error("failed to execute template", "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	})

	// Serve the v2 client library.
	http.Handle("/live.js", live.Javascript{})

	// Start server.
	addr := ":8080"
	log.Printf("Pagination example server starting on http://localhost%s", addr)
	log.Printf("Visit http://localhost%s to see the example", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
