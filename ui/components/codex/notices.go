package codex

import (
	"context"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/a-h/templ"
)

type ChatNotice struct {
	Subject  string
	Action   string
	ObjectID string
	ID       string
	Label    string
	Time     time.Time
	URL      string
}

type (
	NoticeLoader    func(context.Context, string) ([]ChatNotice, error)
	noticeLoaderKey struct{}
)

func WithNoticeLoader(ctx context.Context, load NoticeLoader) context.Context {
	return context.WithValue(ctx, noticeLoaderKey{}, load)
}

func withNotices(in ChatIn) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if load, ok := ctx.Value(automatedLoaderKey{}).(AutomatedLoader); ok && in.SelectedID != "" {
			if automated, err := load(ctx, in.SelectedID); err == nil {
				in.Automated = automated
			}
		}
		if load, ok := ctx.Value(pageLoaderKey{}).(PageLoader); ok && in.SelectedID != "" {
			if pages, err := load(ctx, in.SelectedID); err == nil {
				in.Pages = pages
			}
		}
		if load, ok := ctx.Value(noticeLoaderKey{}).(NoticeLoader); ok && in.SelectedID != "" {
			notices, err := load(ctx, in.SelectedID)
			if err != nil {
				in.NoticeError = "Could not load email activity. Retrying…"
			} else {
				in.Notices = notices
			}
		}
		return withLinks(in, chatContents(in)).Render(ctx, w)
	})
}

func noticeLinkAttributes(ctx context.Context, href string) templ.Attributes {
	out := templ.Attributes{}
	for key, value := range chatLinks(ctx)(href) {
		out[key] = value
	}
	return out
}

func (in ChatIn) draftNotice(href string) (ChatNotice, bool) {
	u, err := url.Parse(href)
	if err != nil || u.Path != "/mail" || in.linkAttributes(href)["href"] == "" {
		return ChatNotice{}, false
	}
	id, account := u.Query().Get("draft"), strings.ToLower(u.Query().Get("account"))
	if id == "" || account == "" {
		return ChatNotice{}, false
	}
	for i := len(in.Notices) - 1; i >= 0; i-- {
		n := in.Notices[i]
		if n.ObjectID == account+"/"+id {
			return n, n.Action == "email_sent" || n.Action == "draft_discarded" || n.Action == "draft_approved" || n.Action == "email_unknown"
		}
	}
	return ChatNotice{}, false
}

func (in ChatIn) noticePositions() [][]ChatNotice {
	positions := make([][]ChatNotice, len(in.Turns)+1)
	notices := append([]ChatNotice(nil), in.Notices...)
	sort.SliceStable(notices, func(i, j int) bool { return notices[i].Time.Before(notices[j].Time) })
	seen := map[string]bool{}
	for _, notice := range notices {
		if notice.Action == "draft_approved" || seen[notice.ID] {
			continue
		}
		seen[notice.ID] = true
		position := len(in.Turns)
		for i, turn := range in.Turns {
			if !turn.StartedAt.IsZero() && notice.Time.Before(turn.StartedAt) {
				position = i
				break
			}
		}
		positions[position] = append(positions[position], notice)
	}
	return positions
}
