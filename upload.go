package live

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// Error sentinels for upload validation failures.
var (
	ErrUploadNotFound     = errors.New("uploads not found")
	ErrUploadTooLarge     = errors.New("upload too large")
	ErrUploadNotAccepted  = errors.New("upload not accepted")
	ErrUploadTooManyFiles = errors.New("upload too many files")
	ErrUploadMalformed    = errors.New("upload malformed")
)

// UploadError wraps an upload sentinel error with optional additional context.
type UploadError struct {
	additional string
	err        error
}

func (u *UploadError) Error() string {
	if u.additional != "" {
		return fmt.Sprintf("%s: %s", u.additional, u.err)
	}
	return fmt.Sprintf("%s", u.err)
}

func (u *UploadError) Unwrap() error {
	return u.err
}

// UploadConfig describes the constraints for a named file upload field.
type UploadConfig struct {
	// Name is the form input name to accept uploads from.
	Name string
	// MaxFiles is the maximum number of files allowed in a single upload.
	MaxFiles int
	// MaxSize is the maximum size in bytes for a single uploaded file.
	MaxSize int64
	// Accept is a list of accepted MIME types (e.g. "image/png").
	Accept []string
}

// Upload describes a single file upload from the client.
type Upload struct {
	// Name is the original filename reported by the client.
	Name string
	// Size is the file size in bytes reported by the client.
	Size int64
	// Type is the MIME type reported by the client.
	Type string
	// LastModified is the last-modified timestamp reported by the client.
	LastModified int64
	// Errors contains any validation errors for this upload.
	Errors []error

	// internalLocation is the path to the staged file on the server.
	// It is unexported so that callers must use File() to access content.
	internalLocation string
}

// File opens and returns an io.ReadCloser for the staged file content.
// The caller is responsible for closing the returned reader.
// Returns an error if internalLocation is empty or the file cannot be opened.
func (u *Upload) File() (io.ReadCloser, error) {
	if u.internalLocation == "" {
		return nil, fmt.Errorf("upload: no staged file location")
	}
	f, err := os.Open(u.internalLocation)
	if err != nil {
		return nil, fmt.Errorf("upload: open staged file: %w", err)
	}
	return f, nil
}

// UploadContext maps upload config names to their list of validated uploads.
type UploadContext map[string][]*Upload

const upKey = "uploads"

// ValidateUploads reads upload metadata from params and validates each file
// against the supplied configs. Validation errors are attached to individual
// Upload entries rather than returned as a function error, so callers receive
// the full context regardless of validation outcome.
//
// params is expected to contain:
//
//	{ "uploads": { "<name>": [ { "name": "...", "size": <int>, "type": "..." }, ... ] } }
func ValidateUploads(params Params, configs []*UploadConfig) (UploadContext, error) {
	ctx := make(UploadContext)

	input, ok := params[upKey].(map[string]any)
	if !ok {
		// No uploads key in params: return empty context without error.
		return ctx, nil
	}

	for _, c := range configs {
		rawList, ok := input[c.Name].([]any)
		if !ok {
			ctx[c.Name] = []*Upload{{Errors: []error{&UploadError{err: ErrUploadNotFound}}}}
			continue
		}

		var uploads []*Upload

		if c.MaxFiles > 0 && len(rawList) > c.MaxFiles {
			// Tag every entry with the too-many-files error.
			for _, raw := range rawList {
				f, _ := raw.(map[string]any)
				u := buildUpload(f)
				u.Errors = append(u.Errors, &UploadError{err: ErrUploadTooManyFiles})
				uploads = append(uploads, u)
			}
			ctx[c.Name] = uploads
			continue
		}

		for _, raw := range rawList {
			f, ok := raw.(map[string]any)
			if !ok {
				uploads = append(uploads, &Upload{Errors: []error{&UploadError{err: ErrUploadMalformed}}})
				continue
			}
			u := buildUpload(f)

			if c.MaxSize > 0 && u.Size > c.MaxSize {
				u.Errors = append(u.Errors, &UploadError{err: ErrUploadTooLarge})
			}

			if len(c.Accept) > 0 {
				accepted := false
				for _, a := range c.Accept {
					if u.Type == a {
						accepted = true
						break
					}
				}
				if !accepted {
					u.Errors = append(u.Errors, &UploadError{err: ErrUploadNotAccepted})
				}
			}

			uploads = append(uploads, u)
		}

		ctx[c.Name] = uploads
	}

	return ctx, nil
}

// buildUpload constructs an Upload from a raw JSON map entry.
func buildUpload(f map[string]any) *Upload {
	if f == nil {
		return &Upload{}
	}
	u := &Upload{
		Name: mapString(f, "name"),
		Type: mapString(f, "type"),
		Size: int64(mapInt(f, "size")),
	}
	// lastModified may arrive as a float64 from JSON decoding.
	if lm, ok := f["lastModified"]; ok {
		switch v := lm.(type) {
		case float64:
			u.LastModified = int64(v)
		case int64:
			u.LastModified = v
		case int:
			u.LastModified = int64(v)
		}
	}
	return u
}

// ConsumeUploads iterates over all uploads registered under name in the
// UploadContext and calls handler for each one. Any errors returned by
// handler are collected and returned. The handler is responsible for
// processing (e.g. moving) the staged file.
func ConsumeUploads(uploads UploadContext, name string, handler func(*Upload) error) []error {
	var errs []error
	list, ok := uploads[name]
	if !ok {
		return errs
	}
	for _, u := range list {
		if err := handler(u); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
