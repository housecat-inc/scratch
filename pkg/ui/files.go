package ui

import "net/url"

func filesDirHref(dir string) string {
	if dir == "" {
		return "/files/"
	}
	return "/files/?dir=" + url.QueryEscape(dir)
}

func filesTreeHref(dir, path string) string {
	return "/files/tree?dir=" + url.QueryEscape(dir) + "&path=" + url.QueryEscape(path)
}
