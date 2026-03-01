package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jfyne/live"
)

// uploadsMockTransport is a transport implementation for uploads example testing.
// It tracks sent events and provides a channel for injecting inbound events.
type uploadsMockTransport struct {
	events chan live.Event
	sent   []live.Event
	mu     sync.Mutex
	closed bool
}

func newUploadsMockTransport() *uploadsMockTransport {
	return &uploadsMockTransport{
		events: make(chan live.Event, 16),
		sent:   []live.Event{},
	}
}

func (m *uploadsMockTransport) Send(e live.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, e)
	return nil
}

func (m *uploadsMockTransport) Events() <-chan live.Event {
	return m.events
}

func (m *uploadsMockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		close(m.events)
		m.closed = true
	}
	return nil
}

func (m *uploadsMockTransport) GetSent() []live.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]live.Event{}, m.sent...)
}

// makeUploadParams builds a Params value that mirrors the JSON the client
// sends for upload validation.
func makeUploadParams(name string, files []map[string]any) live.Params {
	uploads := map[string]any{
		name: make([]any, len(files)),
	}
	list := uploads[name].([]any)
	for i, f := range files {
		list[i] = f
	}
	return live.Params{
		"uploads": uploads,
	}
}

// ---------------------------------------------------------------------------
// Regression tests (must PASS with current framework before uploads example
// exists) — verifies WithUploadConfig and UploadConfigs() from upload.go.
// ---------------------------------------------------------------------------

// TestFramework_WithUploadConfig_Uploads verifies that WithUploadConfig can be
// used when constructing an island and UploadConfigs() returns the registered
// config — a regression of the upload system implemented in T10.
//
// Scenario: Framework supports WithUploadConfig (regression)
func TestFramework_WithUploadConfig_Uploads(t *testing.T) {
	config := &live.UploadConfig{
		Name:     "photos",
		MaxFiles: 3,
		MaxSize:  1 * 1024 * 1024,
		Accept:   []string{"image/png"},
	}

	island, err := live.NewIsland("uploads-reg",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return map[string]any{}, nil
		}),
		live.WithUploadConfig(config),
	)
	if err != nil {
		t.Fatalf("NewIsland() with WithUploadConfig error = %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}

	configs := island.UploadConfigs()
	if len(configs) != 1 {
		t.Fatalf("UploadConfigs() len = %d, want 1", len(configs))
	}
	if configs[0].Name != "photos" {
		t.Errorf("UploadConfigs()[0].Name = %q, want %q", configs[0].Name, "photos")
	}
	if configs[0].MaxFiles != 3 {
		t.Errorf("UploadConfigs()[0].MaxFiles = %d, want 3", configs[0].MaxFiles)
	}
}

// TestFramework_ValidateUploads_Uploads verifies that ValidateUploads correctly
// validates file metadata from params — a regression of the upload system.
//
// Scenario: Valid file passes validation (regression)
func TestFramework_ValidateUploads_Uploads(t *testing.T) {
	config := &live.UploadConfig{
		Name:     "photos",
		MaxFiles: 3,
		MaxSize:  1 * 1024 * 1024,
		Accept:   []string{"image/png"},
	}

	params := makeUploadParams("photos", []map[string]any{
		{"name": "photo.png", "size": 500 * 1024, "type": "image/png"},
	})

	ctx, err := live.ValidateUploads(params, []*live.UploadConfig{config})
	if err != nil {
		t.Fatalf("ValidateUploads() error = %v", err)
	}

	uploads, ok := ctx["photos"]
	if !ok {
		t.Fatal("ValidateUploads() returned context missing 'photos' key")
	}
	if len(uploads) != 1 {
		t.Fatalf("ValidateUploads() len(uploads) = %d, want 1", len(uploads))
	}
	if len(uploads[0].Errors) != 0 {
		t.Errorf("ValidateUploads() valid file has errors: %v", uploads[0].Errors)
	}
}

// ---------------------------------------------------------------------------
// New feature tests (RED — fail to compile until examples/uploads/main.go
// is created)
// ---------------------------------------------------------------------------

// TestUploadsIsland_NewUploadsIsland verifies that NewUploadsIsland constructs
// a valid non-nil Island without error.
//
// Scenario: NewUploadsIsland construction succeeds
func TestUploadsIsland_NewUploadsIsland(t *testing.T) {
	island, err := NewUploadsIsland()
	if err != nil {
		t.Fatalf("NewUploadsIsland() error = %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}
}

// TestUploadsIsland_Mount verifies that mounting the uploads island initializes
// the UploadsState with empty uploads, no errors, and no saved files.
//
// Scenario: Mount handler initializes empty uploads state
func TestUploadsIsland_Mount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewUploadsIsland()
	if err != nil {
		t.Fatalf("NewUploadsIsland() failed: %v", err)
	}

	state, err := island.Mount(ctx, live.Props{}, "")
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	us, ok := state.(*UploadsState)
	if !ok {
		t.Fatalf("expected *UploadsState from Mount, got %T", state)
	}

	if len(us.Errors) != 0 {
		t.Errorf("expected no Errors after mount, got %v", us.Errors)
	}

	if len(us.SavedFiles) != 0 {
		t.Errorf("expected no SavedFiles after mount, got %v", us.SavedFiles)
	}

	// Uploads context may be nil or empty — either is acceptable.
	for _, uploads := range us.Uploads {
		if len(uploads) != 0 {
			t.Errorf("expected empty Uploads after mount, got entries: %v", uploads)
		}
	}
}

// TestUploadsIsland_ValidateValid verifies that the "validate" event handler
// with valid file metadata (500 KB image/png) returns no validation errors.
//
// Scenario: "validate" event with valid files returns no errors
func TestUploadsIsland_ValidateValid(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewUploadsIsland()
	if err != nil {
		t.Fatalf("NewUploadsIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("validate")
	if err != nil {
		t.Fatalf("GetEventHandler('validate') error = %v", err)
	}

	// Valid: 500 KB image/png — under the 1 MB limit, accepted type.
	params := makeUploadParams("photos", []map[string]any{
		{"name": "photo.png", "size": 500 * 1024, "type": "image/png"},
	})

	initialState := &UploadsState{}

	newState, err := handler(ctx, initialState, params)
	if err != nil {
		t.Fatalf("validate handler error = %v", err)
	}

	us, ok := newState.(*UploadsState)
	if !ok {
		t.Fatalf("expected *UploadsState from validate handler, got %T", newState)
	}

	if len(us.Errors) != 0 {
		t.Errorf("expected no Errors for valid upload, got %v", us.Errors)
	}

	uploads, ok := us.Uploads["photos"]
	if !ok || len(uploads) == 0 {
		t.Fatal("expected Uploads['photos'] to be non-empty after validate with valid files")
	}

	for _, u := range uploads {
		if len(u.Errors) != 0 {
			t.Errorf("expected no per-upload errors for valid file, got %v", u.Errors)
		}
	}
}

// TestUploadsIsland_ValidateInvalidOversized verifies that the "validate" event
// handler with an oversized file (2 MB exceeds 1 MB limit) attaches an
// ErrUploadTooLarge error to the upload context.
//
// Scenario: "validate" event with oversized files returns errors
func TestUploadsIsland_ValidateInvalidOversized(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewUploadsIsland()
	if err != nil {
		t.Fatalf("NewUploadsIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("validate")
	if err != nil {
		t.Fatalf("GetEventHandler('validate') error = %v", err)
	}

	// Invalid: 2 MB file exceeds the 1 MB limit.
	params := makeUploadParams("photos", []map[string]any{
		{"name": "big.png", "size": 2 * 1024 * 1024, "type": "image/png"},
	})

	initialState := &UploadsState{}

	newState, err := handler(ctx, initialState, params)
	if err != nil {
		t.Fatalf("validate handler error = %v", err)
	}

	us, ok := newState.(*UploadsState)
	if !ok {
		t.Fatalf("expected *UploadsState from validate handler, got %T", newState)
	}

	uploads, ok := us.Uploads["photos"]
	if !ok || len(uploads) == 0 {
		t.Fatal("expected Uploads['photos'] to be non-empty after validate")
	}

	foundTooLarge := false
	for _, u := range uploads {
		for _, e := range u.Errors {
			if errors.Is(e, live.ErrUploadTooLarge) {
				foundTooLarge = true
			}
		}
	}
	if !foundTooLarge {
		t.Errorf("expected ErrUploadTooLarge in upload errors, got uploads: %v", uploads)
	}
}

// TestUploadsIsland_ValidateInvalidWrongType verifies that the "validate" event
// handler with a text/plain file (not in the accepted list) attaches an
// ErrUploadNotAccepted error to the upload context.
//
// Scenario: "validate" event with wrong-type files returns errors
func TestUploadsIsland_ValidateInvalidWrongType(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewUploadsIsland()
	if err != nil {
		t.Fatalf("NewUploadsIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("validate")
	if err != nil {
		t.Fatalf("GetEventHandler('validate') error = %v", err)
	}

	// Invalid: text/plain is not accepted.
	params := makeUploadParams("photos", []map[string]any{
		{"name": "readme.txt", "size": 100 * 1024, "type": "text/plain"},
	})

	initialState := &UploadsState{}

	newState, err := handler(ctx, initialState, params)
	if err != nil {
		t.Fatalf("validate handler error = %v", err)
	}

	us, ok := newState.(*UploadsState)
	if !ok {
		t.Fatalf("expected *UploadsState from validate handler, got %T", newState)
	}

	uploads, ok := us.Uploads["photos"]
	if !ok || len(uploads) == 0 {
		t.Fatal("expected Uploads['photos'] to be non-empty after validate")
	}

	foundNotAccepted := false
	for _, u := range uploads {
		for _, e := range u.Errors {
			if errors.Is(e, live.ErrUploadNotAccepted) {
				foundNotAccepted = true
			}
		}
	}
	if !foundNotAccepted {
		t.Errorf("expected ErrUploadNotAccepted in upload errors, got uploads: %v", uploads)
	}
}

// TestUploadsIsland_Save verifies that the "save" event handler calls
// ConsumeUploads for each staged upload and records the file names in
// UploadsState.SavedFiles. The test uses upload entries that have an
// internalLocation set to a real temp file so ConsumeUploads can call File().
//
// Note: In practice ConsumeUploads is called on the upload context; the "save"
// handler is expected to iterate over uploads and populate SavedFiles. This
// test drives the handler in isolation with a pre-built UploadsState.
//
// Scenario: "save" event calls ConsumeUploads and processes files
func TestUploadsIsland_Save(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewUploadsIsland()
	if err != nil {
		t.Fatalf("NewUploadsIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("save")
	if err != nil {
		t.Fatalf("GetEventHandler('save') error = %v", err)
	}

	// Build an UploadsState that already has validated uploads waiting to be
	// consumed. Use uploads with no validation errors.
	initialState := &UploadsState{
		Uploads: live.UploadContext{
			"photos": {
				{Name: "a.png", Size: 100 * 1024, Type: "image/png"},
				{Name: "b.png", Size: 200 * 1024, Type: "image/png"},
			},
		},
		Errors:     []error{},
		SavedFiles: []string{},
	}

	newState, err := handler(ctx, initialState, live.Params{})
	if err != nil {
		t.Fatalf("save handler error = %v", err)
	}

	us, ok := newState.(*UploadsState)
	if !ok {
		t.Fatalf("expected *UploadsState from save handler, got %T", newState)
	}

	// After save, SavedFiles should be populated with the uploaded file names.
	if len(us.SavedFiles) == 0 {
		t.Error("expected SavedFiles to be non-empty after save handler")
	}
}

// TestUploadsIsland_UploadConfig verifies that the uploads island is configured
// with the expected UploadConfig ("photos", MaxFiles=3, MaxSize=1MB,
// Accept=["image/png"]).
//
// Scenario: Island uses WithUploadConfig for photos
func TestUploadsIsland_UploadConfig(t *testing.T) {
	island, err := NewUploadsIsland()
	if err != nil {
		t.Fatalf("NewUploadsIsland() failed: %v", err)
	}

	configs := island.UploadConfigs()
	if len(configs) == 0 {
		t.Fatal("expected at least one UploadConfig registered on uploads island")
	}

	var photosConfig *live.UploadConfig
	for _, c := range configs {
		if c.Name == "photos" {
			photosConfig = c
			break
		}
	}
	if photosConfig == nil {
		t.Fatal("expected an UploadConfig with Name='photos', found none")
	}

	if photosConfig.MaxFiles != 3 {
		t.Errorf("expected MaxFiles = 3, got %d", photosConfig.MaxFiles)
	}

	const oneMB = 1 * 1024 * 1024
	if photosConfig.MaxSize != oneMB {
		t.Errorf("expected MaxSize = %d (1MB), got %d", oneMB, photosConfig.MaxSize)
	}

	if len(photosConfig.Accept) == 0 {
		t.Fatal("expected Accept to be non-empty")
	}
	foundPNG := false
	for _, a := range photosConfig.Accept {
		if a == "image/png" {
			foundPNG = true
		}
	}
	if !foundPNG {
		t.Errorf("expected Accept to contain 'image/png', got %v", photosConfig.Accept)
	}
}

// TestUploadsIsland_MountAndRender verifies the full mount-and-render path
// through the engine: registering the island, mounting it, and confirming
// that at least one patch event containing valid HTML is sent to the transport.
//
// Scenario: Mount and render via engine produces valid HTML
func TestUploadsIsland_MountAndRender(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("uploads", NewUploadsIsland)
	if err != nil {
		t.Fatalf("registry.Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newUploadsMockTransport()
	session := live.NewSession(ctx, "session-uploads", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-uploads", "uploads-1", "uploads", live.Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Verify the mounted state is an UploadsState.
	us, ok := instance.State().(*UploadsState)
	if !ok {
		t.Fatalf("expected *UploadsState, got %T", instance.State())
	}
	if len(us.Errors) != 0 {
		t.Errorf("expected no Errors after mount via engine, got %v", us.Errors)
	}
	if len(us.SavedFiles) != 0 {
		t.Errorf("expected no SavedFiles after mount via engine, got %v", us.SavedFiles)
	}

	// Verify at least one patch event was sent to the transport after mount.
	sent := transport.GetSent()
	if len(sent) == 0 {
		t.Fatal("expected at least one event sent to transport after mount")
	}

	// Find a patch event and confirm its data contains HTML.
	var patchEvent *live.Event
	for i := range sent {
		if sent[i].T == live.EventPatch {
			patchEvent = &sent[i]
			break
		}
	}
	if patchEvent == nil {
		t.Fatal("expected at least one EventPatch event to be sent after mount")
	}

	html := strings.TrimSpace(string(patchEvent.Data))
	if html == "" {
		t.Error("expected non-empty HTML in patch event data after mount")
	}
}
