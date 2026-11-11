package htmx


import (
	"embed"
	"io/fs"
)

// htmx.min.js is vendored htmx 4.0.0.
//
//go:embed htmx.min.js
var bundle embed.FS

type assets struct {
	fs.FS
}

func (a assets) Open(name string) (fs.File, error) {
	if name == "js/htmx.min.js" {
		return bundle.Open("htmx.min.js")
	}
	return a.FS.Open(name)
}

func Assets(base fs.FS) fs.FS {
	return assets{FS: base}
}
