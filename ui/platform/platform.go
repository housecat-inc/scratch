// Package platform tells a Mac from everything else, from the request, so a
// page can show ⌘ or Ctrl in its first byte of HTML. CSS cannot detect the
// operating system, and deciding in script would render the wrong key until
// the script ran.
package platform

import (
	"context"
	"net/http"
	"strings"
)

type Platform string

const (
	Mac   Platform = "mac"
	Other Platform = "other"
)

// FromRequest prefers the Sec-CH-UA-Platform client hint, which Chromium sends
// on every request and states the platform outright, and falls back to the
// User-Agent for Safari and Firefox, which do not send it. iPhone and iPad
// count as Mac: their keyboards shortcut with ⌘, and iPadOS Safari reports a
// Macintosh user agent anyway.
func FromRequest(r *http.Request) Platform {
	if hint := strings.Trim(r.Header.Get("Sec-CH-UA-Platform"), `"`); hint != "" {
		if hint == "macOS" || hint == "iOS" {
			return Mac
		}
		return Other
	}
	ua := r.Header.Get("User-Agent")
	for _, marker := range []string{"Macintosh", "iPhone", "iPad"} {
		if strings.Contains(ua, marker) {
			return Mac
		}
	}
	return Other
}

type contextKey struct{}

func WithContext(ctx context.Context, p Platform) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

// FromContext reads the platform a request was marked with, and reads Other
// when it was never marked, so an unmarked render shows Ctrl.
func FromContext(ctx context.Context) Platform {
	if p, ok := ctx.Value(contextKey{}).(Platform); ok {
		return p
	}
	return Other
}

// Middleware marks each request's context with its platform. The response
// varies on the headers the platform was read from, so no cache hands one
// platform's page to another.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Sec-CH-UA-Platform, User-Agent")
		next.ServeHTTP(w, r.WithContext(WithContext(r.Context(), FromRequest(r))))
	})
}
