package codex

import (
	"context"
	"net/url"
	"strings"

	"github.com/a-h/templ"
)

func pageContextSlot(in ChatIn) templ.Component {
	if in.LinkTarget == "" {
		return nil
	}
	return pageContext(in)
}

func pageContextURL(in ChatIn) string {
	u, err := url.Parse(in.PageURL)
	if err != nil || in.linkAttributes(in.PageURL)["href"] == "" {
		return ""
	}
	u.Scheme, u.Host = "", ""
	q := u.Query()
	q.Del("chat")
	q.Del("_partial")
	u.RawQuery = q.Encode()
	return u.String()
}

func contextDetail(value string) string {
	if value == "" {
		return ""
	}
	return " · " + value
}

func MailContextKey(href string, origins []string) string {
	in := ChatIn{PageURL: href, LinkPath: "/mail", LinkOrigins: origins}
	normalized := pageContextURL(in)
	if normalized == "" {
		return ""
	}
	u, err := url.Parse(normalized)
	if err != nil {
		return ""
	}
	u.Fragment, u.RawFragment = "", ""
	q := u.Query()
	account := strings.ToLower(strings.TrimSpace(q.Get("account")))
	if u.Path != "/mail" && u.Path != "/mail/" {
		q = url.Values{}
	} else {
		u.Path = "/mail"
		q.Del("back")
	}
	if account != "" {
		q.Set("account", account)
	} else {
		q.Del("account")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

type (
	lastContextKey      struct{}
	LastContextResolver func(context.Context, string) string
)

func WithLastContext(ctx context.Context, resolve LastContextResolver) context.Context {
	return context.WithValue(ctx, lastContextKey{}, resolve)
}

func lastContext(ctx context.Context, in ChatIn) string {
	resolve, _ := ctx.Value(lastContextKey{}).(LastContextResolver)
	if resolve == nil {
		return ""
	}
	return resolve(ctx, in.SelectedID)
}

// ContextBaseline is the codex:context-baseline event's detail, which the
// chat's queue sends in its HX-Trigger header: the mail context key the
// thread last sent, the one the page context hides its chip for.
type ContextBaseline struct {
	Key    string `json:"key"`
	Thread string `json:"thread"`
}
