package ui

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/cockroachdb/errors"
)

//go:embed static
var staticFS embed.FS

func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(errors.Wrap(err, "static fs"))
	}
	return http.FileServer(http.FS(sub))
}
