package app

import (
	"context"

	"github.com/a-h/templ"
)

type Rail struct {
	Chat templ.Component
	Open bool
}

type railKey struct{}

func WithRail(ctx context.Context, rail Rail) context.Context {
	return context.WithValue(ctx, railKey{}, rail)
}

func RailFrom(ctx context.Context) (Rail, bool) {
	rail, ok := ctx.Value(railKey{}).(Rail)
	return rail, ok
}
