package ui

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

type chatSidebarKey struct{}

func WithChatSidebar(ctx context.Context, load func(int64) (ChatSidebarProps, error)) context.Context {
	return context.WithValue(ctx, chatSidebarKey{}, load)
}

func initialChatSidebar(ctx context.Context) templ.Component {
	load, ok := ctx.Value(chatSidebarKey{}).(func(int64) (ChatSidebarProps, error))
	if !ok {
		return nil
	}
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		props, err := load(0)
		if err != nil {
			return err
		}
		return ChatSidebarChats(props).Render(ctx, w)
	})
}
