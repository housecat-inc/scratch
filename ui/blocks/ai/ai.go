// Package ai holds the chat block. Its only other file is a .templ, and
// generated code is gitignored, so without this file the directory is not a Go
// package at all: `go generate ./...` cannot match it, nothing generates the
// templ, and the import fails to resolve on a fresh checkout. It builds locally
// only because the generated file is already on disk.
//
//go:generate go tool templ generate
package ai
