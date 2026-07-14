package ui

import (
	"net/url"
	"path"
)

func filesDirHref(dir string) string {
	if dir == "" {
		return "/files/"
	}
	return "/files/?dir=" + url.QueryEscape(dir)
}

func filesChildDirHref(dir, name string) string {
	return filesDirHref(path.Join(dir, name))
}

func filesTreeHref(dir, path string) string {
	return "/files/tree?dir=" + url.QueryEscape(dir) + "&path=" + url.QueryEscape(path)
}

func dirName(dir string) string {
	if dir == "" || dir == "/" {
		return "/"
	}
	return path.Base(dir)
}
