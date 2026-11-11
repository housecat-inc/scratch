// Package assets serves what the ui module adds to the app's /assets: htmx.
// ui.ts here is the entry point of ui.js, the one script that brings every
// component's behavior, which the one Vite+ build of every bundle builds from
// bundle/ and app/bundles serves.
package assets

import (
	"io"
	"io/fs"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/housecat-inc/scratch/ui/htmx"
)

// embed.FS reports the zero time for every file it holds, and ServeContent
// omits Last-Modified rather than send one, so a browser has nothing to
// revalidate against and re-downloads the file on every navigation. The
// binary's own start is the truest mtime an embedded file has; a file read off
// disk in dev already carries a real one and keeps it.
var buildTime = time.Now()

type stamped struct {
	fs.FS
}

func (s stamped) Open(name string) (fs.File, error) {
	f, err := s.FS.Open(name)
	if err != nil {
		return nil, errors.Wrap(err, "open")
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, errors.Wrap(err, "stat")
	}
	// A directory is wrapped away from its ReadDir, and ServeContent never sees
	// one anyway.
	if info.IsDir() || !info.ModTime().IsZero() {
		return f, nil
	}
	return stampedFile{File: f}, nil
}

type stampedFile struct {
	fs.File
}

func (f stampedFile) Seek(offset int64, whence int) (int64, error) {
	seeker, ok := f.File.(io.Seeker)
	if !ok {
		return 0, errors.New("file does not seek")
	}
	n, err := seeker.Seek(offset, whence)
	return n, errors.Wrap(err, "seek")
}

func (f stampedFile) Stat() (fs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, errors.Wrap(err, "stat")
	}
	return stampedInfo{FileInfo: info}, nil
}

type stampedInfo struct {
	fs.FileInfo
}

func (i stampedInfo) ModTime() time.Time { return buildTime }

// FS serves the ui module's files over whatever the app serves under /assets:
// htmx at js/htmx.min.js.
func FS(base fs.FS) fs.FS {
	return stamped{FS: htmx.Assets(base)}
}
