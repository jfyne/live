package live

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

const upKey = "uploads"

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

var (
	ErrUploadNotFound     = errors.New("uploads not found")
	ErrUploadTooLarge     = errors.New("upload too large")
	ErrUploadNotAccepted  = errors.New("upload not accepted")
	ErrUploadTooManyFiles = errors.New("upload too many files")
	ErrUploadMalformed    = errors.New("upload malformed")
)

// UploadConfig describes an upload to accept on the socket.
type UploadConfig struct {
	// The form input name to accept from.
	Name string
	// The max number of files to allow to be uploaded.
	MaxFiles int
	// The maximum size of all files to accept.
	MaxSize int64
	// Which type of files to accept.
	Accept []string
}

// Upload describes an upload from the client.
type Upload struct {
	Name         string
	Size         int64
	Type         string
	LastModified string
	Errors       []error
	Progress     float32

	internalLocation string `json:"-"`
	bytesRead        int64  `json:"-"`
}

// File gets an open file reader.
func (u Upload) File() (*os.File, error) {
	return os.Open(u.internalLocation)
}

// UploadContext the context which we render to templates.
type UploadContext map[string][]*Upload

// HasErrors does the upload context have any errors.
func (u UploadContext) HasErrors() bool {
	for _, uploads := range u {
		for _, u := range uploads {
			if len(u.Errors) > 0 {
				return true
			}
		}
	}
	return false
}

// UploadProgress tracks uploads and updates an upload
// object with progress.
type UploadProgress struct {
	Upload *Upload
	Engine *Engine
	Socket *Socket
}

// Write interface to track progress of an upload.
func (u *UploadProgress) Write(p []byte) (n int, err error) {
	n = len(p)
	u.Upload.bytesRead += int64(n)
	u.Upload.Progress = float32(u.Upload.bytesRead) / float32(u.Upload.Size)
	if rerr := u.Socket.Render(context.Background()); rerr != nil {
		slog.Error("error in upload progress", "err", rerr)
	}
	return
}

// ValidateUploads checks proposed uploads for errors, should be called
// in a validation check function.
func ValidateUploads(s *Socket, p Params) {
	s.ClearUploads()

	input, ok := p[upKey].(map[string]any)
	if !ok {
		slog.Warn("validate uploads", "err", ErrUploadNotFound)
		return
	}

	for _, c := range s.UploadConfigs() {
		uploads, ok := input[c.Name].([]any)
		if !ok {
			s.AssignUpload(c.Name, &Upload{Errors: []error{ErrUploadNotFound}})
			continue
		}
		if len(uploads) > c.MaxFiles {
			s.AssignUpload(c.Name, &Upload{Errors: []error{&UploadError{err: ErrUploadTooManyFiles}}})
		}
		for _, u := range uploads {
			f, ok := u.(map[string]any)
			if !ok {
				s.AssignUpload(c.Name, &Upload{Errors: []error{&UploadError{err: ErrUploadNotFound}}})
				continue
			}
			u := &Upload{
				Name: mapString(f, "name"),
				Size: int64(mapInt(f, "size")),
				Type: mapString(f, "type"),
			}

			// Check size.
			if u.Size > c.MaxSize {
				u.Errors = append(u.Errors, &UploadError{err: ErrUploadTooLarge})
			}

			// Check Accept.
			accepted := false
			for _, a := range c.Accept {
				if u.Type == a {
					accepted = true
				}
			}
			if !accepted {
				u.Errors = append(u.Errors, &UploadError{err: ErrUploadNotAccepted})
			}
			s.AssignUpload(c.Name, u)
		}
	}
}

// ConsumeHandler callback type when uploads are consumed.
type ConsumeHandler func(u *Upload) error

// ConsumeUploads helper function to consume the staged uploads.
func ConsumeUploads(s *Socket, name string, ch ConsumeHandler) []error {
	errs := []error{}
	all := s.Uploads()
	uploads, ok := all[name]
	if !ok {
		return errs
	}
	for _, u := range uploads {
		if err := ch(u); err != nil {
			errs = append(errs, err)
		}
		s.ClearUpload(name, u)
	}
	return errs
}

// WithMaxUploadSize set the handler engine to have a maximum upload size.
func WithMaxUploadSize(size int64) EngineConfig {
	return func(e *Engine) error {
		e.MaxUploadSize = size
		return nil
	}
}

// WithUploadStagingLocation set the handler engine with a specific upload staging location.
func WithUploadStagingLocation(stagingLocation string) EngineConfig {
	return func(e *Engine) error {
		e.UploadStagingLocation = stagingLocation
		return nil
	}
}
