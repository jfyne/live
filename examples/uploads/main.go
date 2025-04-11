package main

import (
	"context"
	"errors"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfyne/live"
)

const (
	validate = "validate"
	save     = "save"
)

type model struct {
	Uploads []string
}

func newModel(s *live.Socket) *model {
	m, ok := s.Assigns().(*model)
	if !ok {
		return &model{
			Uploads: []string{},
		}
	}
	return m
}

// customError formats upload validation errors.
func customError(u *live.Upload, err error) string {
	msg := []string{}
	if u.Name != "" {
		msg = append(msg, u.Name)
	}
	switch {
	case errors.Is(err, live.ErrUploadTooLarge):
		msg = append(msg, "This is a custom too large message: "+err.Error())
	case errors.Is(err, live.ErrUploadTooManyFiles):
		msg = append(msg, "This is a custom too many files message: "+err.Error())
	default:
		msg = append(msg, err.Error())
	}
	return strings.Join(msg, " - ")
}

func main() {

	// Setup the template with some funcs to provide custom error messages.
	t, err := template.New("root.html").Funcs(template.FuncMap{
		"customError": customError,
	}).ParseFiles("root.html", "uploads/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h := live.NewHandler(live.WithTemplateRenderer(t))

	// In the mount function we call `AllowUploads` on the socket which configures
	// what is allowed to be uploaded.
	h.MountHandler = func(ctx context.Context, s *live.Socket) (any, error) {
		s.AllowUploads(&live.UploadConfig{
			// Name refers to the name of the file input field.
			Name: "photos",
			// We are accepting a maximum of 3 files.
			MaxFiles: 3,
			// For each of those files we are only allowing them to be 1MB.
			MaxSize: 1 * 1024 * 1024,
			// We are only accepting .png.
			Accept: []string{"image/png"},
		})
		return newModel(s), nil
	}

	// On form change we perform validation.
	h.HandleEvent(validate, func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
		m := newModel(s)
		// This helper function populates the socket `Uploads` with errors.
		live.ValidateUploads(s, p)
		return m, nil
	})

	// On form save, the client first posts the files then this event handler is called.
	// Here we can to consume the files from our staging area.
	h.HandleEvent(save, func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
		m := newModel(s)

		// `ConsumeUploads` helper function is used to iterate over the "photos" input files
		// that have been uploaded.
		live.ConsumeUploads(s, "photos", func(u *live.Upload) error {
			// First we get the staged file.
			file, err := u.File()
			if err != nil {
				return err
			}
			// When we are done close the file, and remove it from staging.
			defer func() {
				file.Close()
				os.Remove(file.Name())
			}()

			// Create a new file in our static directory to copy the staged file into.
			dst, err := os.Create(filepath.Join("uploads", "static", u.Name))
			if err != nil {
				return err
			}
			defer dst.Close()

			// Do the copy
			if _, err := io.Copy(dst, file); err != nil {
				return err
			}

			// Record the name of the file so we can show the link to it.
			m.Uploads = append(m.Uploads, u.Name)

			return nil
		})

		return m, nil
	})

	http.Handle("/", live.NewHttpHandler(
		context.Background(),
		h,
		// Only allow a total of 10MBs to be uploaded.
		live.WithMaxUploadSize(10*1024*1024)))

	// Set up the static file handling for the uploads we have consumed.
	fs := http.FileServer(http.Dir(filepath.Join("uploads", "static")))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
