// Package wire holds the JSON shapes the shared UI's scripts read from the
// server, so ui/cmd/browserschemas can generate their Schemas. It imports
// nothing that reaches the bundles' build (app/pkg/browserschemas).
package wire

// ErrorMessage is an echo route's error answer. echo's error handler sends
// it for an echo.HTTPError, and a handler that answers an error with JSON
// itself sends the same.
type ErrorMessage struct {
	Message string `json:"message"`
}
