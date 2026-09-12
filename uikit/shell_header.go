package uikit

import (
	"context"
	"github.com/a-h/templ"
)

type shellHeaderKey struct{}

func WithShellHeader(ctx context.Context, header templ.Component) context.Context {
	return context.WithValue(ctx, shellHeaderKey{}, header)
}

func ShellHeader(ctx context.Context) templ.Component {
	header, _ := ctx.Value(shellHeaderKey{}).(templ.Component)
	return header
}
