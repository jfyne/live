package live

// ---------------------------------------------------------------------------
// RED tests for the upload system (Task 24)
// ---------------------------------------------------------------------------
// These tests reference types and functions that do not yet exist in the v2
// branch and will fail to compile until the implementation is added.
//
// Reference implementation (master branch): upload.go
//
// Types / functions expected by these tests:
//   - UploadConfig        struct{Name string; MaxFiles int; MaxSize int64; Accept []string}
//   - Upload              struct{Name, Type string; Size int64; LastModified string; Errors []error}
//   - UploadContext       type map[string][]*Upload
//   - ValidateUploads     func(params Params, configs []*UploadConfig) (UploadContext, error)
//   - ConsumeUploads      func(uploads UploadContext, name string, handler func(*Upload) error) []error
//   - Upload.File()       func() (io.Reader, error)
//   - WithUploadConfig    func(config *UploadConfig) IslandConfig
//   - Island.UploadConfigs() []*UploadConfig
//   - Error sentinels: ErrUploadTooLarge, ErrUploadNotAccepted, ErrUploadTooManyFiles
// ---------------------------------------------------------------------------

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeUploadParams builds a Params value that mirrors the JSON the client
// sends for upload validation (see forms.ts serialize()).
func makeUploadParams(name string, files []map[string]any) Params {
	uploads := map[string]any{
		name: make([]any, len(files)),
	}
	list := uploads[name].([]any)
	for i, f := range files {
		list[i] = f
	}
	return Params{
		"uploads": uploads,
	}
}

// ---------------------------------------------------------------------------
// Scenario: valid file passes validation
// Given an UploadConfig allowing max 1 MB and image/png
// And a Params containing a single 500 KB image/png file
// When ValidateUploads is called
// Then the returned UploadContext contains the file with no errors
// ---------------------------------------------------------------------------

func TestValidateUploads_Valid(t *testing.T) {
	const halfMB = 500 * 1024

	config := &UploadConfig{
		Name:     "avatar",
		MaxFiles: 1,
		MaxSize:  1 * 1024 * 1024, // 1 MB
		Accept:   []string{"image/png"},
	}

	params := makeUploadParams("avatar", []map[string]any{
		{"name": "photo.png", "size": halfMB, "type": "image/png"},
	})

	ctx, err := ValidateUploads(params, []*UploadConfig{config})
	if err != nil {
		t.Fatalf("ValidateUploads() error = %v, want nil", err)
	}

	uploads, ok := ctx["avatar"]
	if !ok {
		t.Fatal("ValidateUploads() returned context missing 'avatar' key")
	}
	if len(uploads) != 1 {
		t.Fatalf("ValidateUploads() len(uploads) = %d, want 1", len(uploads))
	}
	if len(uploads[0].Errors) != 0 {
		t.Errorf("ValidateUploads() valid file has errors: %v", uploads[0].Errors)
	}
}

// ---------------------------------------------------------------------------
// Scenario: file too large fails with ErrUploadTooLarge
// Given an UploadConfig allowing max 1 MB
// And a Params containing a 2 MB file
// When ValidateUploads is called
// Then the upload has an error wrapping ErrUploadTooLarge
// ---------------------------------------------------------------------------

func TestValidateUploads_TooLarge(t *testing.T) {
	const twoMB = 2 * 1024 * 1024

	config := &UploadConfig{
		Name:     "avatar",
		MaxFiles: 1,
		MaxSize:  1 * 1024 * 1024, // 1 MB
		Accept:   []string{"image/png"},
	}

	params := makeUploadParams("avatar", []map[string]any{
		{"name": "big.png", "size": twoMB, "type": "image/png"},
	})

	ctx, err := ValidateUploads(params, []*UploadConfig{config})
	if err != nil {
		t.Fatalf("ValidateUploads() error = %v, want nil (errors are in upload context)", err)
	}

	uploads, ok := ctx["avatar"]
	if !ok {
		t.Fatal("ValidateUploads() returned context missing 'avatar' key")
	}
	if len(uploads) == 0 {
		t.Fatal("ValidateUploads() no uploads returned")
	}

	found := false
	for _, e := range uploads[0].Errors {
		if errors.Is(e, ErrUploadTooLarge) {
			found = true
		}
	}
	if !found {
		t.Errorf("ValidateUploads() expected ErrUploadTooLarge in errors, got: %v", uploads[0].Errors)
	}
}

// ---------------------------------------------------------------------------
// Scenario: wrong file type fails with ErrUploadNotAccepted
// Given an UploadConfig accepting only image/png
// And a Params containing a text/plain file
// When ValidateUploads is called
// Then the upload has an error wrapping ErrUploadNotAccepted
// ---------------------------------------------------------------------------

func TestValidateUploads_WrongType(t *testing.T) {
	const smallFile = 100 * 1024

	config := &UploadConfig{
		Name:     "avatar",
		MaxFiles: 1,
		MaxSize:  1 * 1024 * 1024,
		Accept:   []string{"image/png"},
	}

	params := makeUploadParams("avatar", []map[string]any{
		{"name": "readme.txt", "size": smallFile, "type": "text/plain"},
	})

	ctx, err := ValidateUploads(params, []*UploadConfig{config})
	if err != nil {
		t.Fatalf("ValidateUploads() error = %v, want nil (errors are in upload context)", err)
	}

	uploads, ok := ctx["avatar"]
	if !ok {
		t.Fatal("ValidateUploads() returned context missing 'avatar' key")
	}
	if len(uploads) == 0 {
		t.Fatal("ValidateUploads() no uploads returned")
	}

	found := false
	for _, e := range uploads[0].Errors {
		if errors.Is(e, ErrUploadNotAccepted) {
			found = true
		}
	}
	if !found {
		t.Errorf("ValidateUploads() expected ErrUploadNotAccepted in errors, got: %v", uploads[0].Errors)
	}
}

// ---------------------------------------------------------------------------
// Scenario: too many files fails with ErrUploadTooManyFiles
// Given an UploadConfig allowing max 3 files
// And a Params containing 5 files
// When ValidateUploads is called
// Then the UploadContext contains an error wrapping ErrUploadTooManyFiles
// ---------------------------------------------------------------------------

func TestValidateUploads_TooManyFiles(t *testing.T) {
	config := &UploadConfig{
		Name:     "photos",
		MaxFiles: 3,
		MaxSize:  1 * 1024 * 1024,
		Accept:   []string{"image/png"},
	}

	files := make([]map[string]any, 5)
	for i := range files {
		files[i] = map[string]any{
			"name": "photo.png",
			"size": 100 * 1024,
			"type": "image/png",
		}
	}

	params := makeUploadParams("photos", files)

	ctx, err := ValidateUploads(params, []*UploadConfig{config})
	if err != nil {
		t.Fatalf("ValidateUploads() error = %v, want nil (errors are in upload context)", err)
	}

	uploads := ctx["photos"]

	found := false
	for _, u := range uploads {
		for _, e := range u.Errors {
			if errors.Is(e, ErrUploadTooManyFiles) {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("ValidateUploads() expected ErrUploadTooManyFiles, got uploads: %v", uploads)
	}
}

// ---------------------------------------------------------------------------
// Scenario: ConsumeUploads calls handler for each file in the upload context
// Given an UploadContext with two uploads under key "docs"
// When ConsumeUploads is called with a handler that appends names
// Then the handler is called once per upload
// ---------------------------------------------------------------------------

func TestConsumeUploads(t *testing.T) {
	uploads := UploadContext{
		"docs": {
			{Name: "a.png", Size: 1024, Type: "image/png"},
			{Name: "b.png", Size: 2048, Type: "image/png"},
		},
	}

	var consumed []string
	errs := ConsumeUploads(uploads, "docs", func(u *Upload) error {
		consumed = append(consumed, u.Name)
		return nil
	})

	if len(errs) != 0 {
		t.Errorf("ConsumeUploads() errors = %v, want none", errs)
	}
	if len(consumed) != 2 {
		t.Fatalf("ConsumeUploads() handler called %d times, want 2", len(consumed))
	}
	if consumed[0] != "a.png" {
		t.Errorf("ConsumeUploads() consumed[0] = %q, want %q", consumed[0], "a.png")
	}
	if consumed[1] != "b.png" {
		t.Errorf("ConsumeUploads() consumed[1] = %q, want %q", consumed[1], "b.png")
	}
}

// ---------------------------------------------------------------------------
// Scenario: WithUploadConfig adds config to island; UploadConfigs() returns it
// Given a new island
// When WithUploadConfig is used as an IslandConfig
// Then island.UploadConfigs() returns the registered config
// ---------------------------------------------------------------------------

func TestUploadConfig_OnIsland(t *testing.T) {
	config := &UploadConfig{
		Name:     "avatar",
		MaxFiles: 1,
		MaxSize:  1 * 1024 * 1024,
		Accept:   []string{"image/png"},
	}

	island, err := NewIsland("uploader", WithUploadConfig(config))
	if err != nil {
		t.Fatalf("NewIsland() with WithUploadConfig error = %v", err)
	}

	configs := island.UploadConfigs()
	if len(configs) != 1 {
		t.Fatalf("island.UploadConfigs() len = %d, want 1", len(configs))
	}
	if configs[0].Name != "avatar" {
		t.Errorf("island.UploadConfigs()[0].Name = %q, want %q", configs[0].Name, "avatar")
	}
	if configs[0].MaxSize != config.MaxSize {
		t.Errorf("island.UploadConfigs()[0].MaxSize = %d, want %d", configs[0].MaxSize, config.MaxSize)
	}
}

// ---------------------------------------------------------------------------
// Scenario: Upload.File() returns an io.Reader for the staged file content
// Given an Upload whose internalLocation points to a temp file containing data
// When Upload.File() is called
// Then an io.Reader is returned that yields the file content
// ---------------------------------------------------------------------------

func TestUpload_File(t *testing.T) {
	// Create a temporary file to simulate a staged upload.
	content := []byte("hello upload")
	tmp, err := os.CreateTemp("", "upload-test-*.bin")
	if err != nil {
		t.Fatalf("os.CreateTemp() error = %v", err)
	}
	defer os.Remove(tmp.Name())

	if _, werr := tmp.Write(content); werr != nil {
		t.Fatalf("tmp.Write() error = %v", werr)
	}
	if cerr := tmp.Close(); cerr != nil {
		t.Fatalf("tmp.Close() error = %v", cerr)
	}

	// Build an Upload that points at the temp file.
	// The new API returns io.ReadCloser rather than *os.File.
	u := &Upload{
		Name:             "test.bin",
		Size:             int64(len(content)),
		Type:             "application/octet-stream",
		internalLocation: tmp.Name(),
	}

	r, err := u.File()
	if err != nil {
		t.Fatalf("Upload.File() error = %v", err)
	}
	defer r.Close()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}

	if !bytes.Equal(got, content) {
		t.Errorf("Upload.File() content = %q, want %q", got, content)
	}
}
