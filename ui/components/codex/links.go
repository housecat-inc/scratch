package codex

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/housecat-inc/scratch/ui/uikit"
)

type (
	linksKey          struct{}
	linkNormalizerKey struct{}
)

func WithLinkNormalizer(ctx context.Context, normalize func(string) string) context.Context {
	return context.WithValue(ctx, linkNormalizerKey{}, normalize)
}

func normalizedLink(ctx context.Context, href string) string {
	if normalize, ok := ctx.Value(linkNormalizerKey{}).(func(string) string); ok {
		return normalize(href)
	}
	return href
}

func withLinks(in ChatIn, content templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		ctx, cancel := linkRenderContext(ctx)
		defer cancel()
		return content.Render(context.WithValue(ctx, linksKey{}, in), w)
	})
}

func chatLinks(ctx context.Context) uikit.LinkAttributes {
	in, _ := ctx.Value(linksKey{}).(ChatIn)
	return func(href string) map[string]string {
		return in.linkAttributes(normalizedLink(ctx, href))
	}
}

func (in ChatIn) linkAttributes(href string) map[string]string {
	u, err := url.Parse(href)
	if err != nil || (u.Scheme != "" && !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) {
		return nil
	}
	allowedOrigin := slices.ContainsFunc(in.LinkOrigins, func(origin string) bool {
		return strings.EqualFold(origin, u.Scheme+"://"+u.Host)
	})
	internal := u.User == nil && ((u.Scheme == "" && u.Host == "") || allowedOrigin)
	if !internal {
		return map[string]string{"target": "_blank", "rel": "nofollow noopener noreferrer"}
	}
	matches := func(prefix string) bool {
		return prefix != "" && (u.Path == prefix || strings.HasPrefix(u.Path, prefix+"/"))
	}
	if !matches(in.LinkPath) && !slices.ContainsFunc(in.LinkPaths, matches) {
		return nil
	}
	if u.Query().Get("download") == "1" {
		return nil
	}
	selected := in.SelectedID
	if selected == "" {
		selected = "new"
	}
	query := u.Query()
	inherited := !query.Has("chat")
	if inherited {
		query.Set("chat", selected)
	}
	u.Scheme = ""
	u.Host = ""
	u.RawQuery = query.Encode()
	out := map[string]string{"href": u.String()}
	if inherited {
		out["data-chat-link"] = "inherited"
	}
	if in.LinkTarget != "" && query.Get("chat") == selected {
		out["hx-push-url"] = u.String()
		out["hx-target"] = in.LinkTarget
		out["hx-swap"] = "innerHTML"
		query.Set("_partial", "1")
		u.RawQuery = query.Encode()
		u.Fragment = ""
		u.RawFragment = ""
		out["hx-get"] = u.String()
	}
	return out
}

type (
	// PageLoader reads, by client id, the page each message of a chat was
	// sent from (ChatIn.Pages).
	PageLoader    func(context.Context, string) (map[string]string, error)
	pageLoaderKey struct{}
)

func WithPageLoader(ctx context.Context, load PageLoader) context.Context {
	return context.WithValue(ctx, pageLoaderKey{}, load)
}

// A chat's history version is how it records its messages (ChatIn.History).
// Housecat keeps it for the chats it starts; any other chat is
// HistoryPagesInText.
const (
	// HistoryPagesInText chats carry a message's page at the end of its
	// text: a "Current page:" line and, for a mail list, its description
	// quoted as a JSON string (SplitPageContext). Chats from before history
	// versions were kept are this version.
	HistoryPagesInText = 1
	// HistoryPagesApart chats hold only what the user typed in a message and
	// keep its page beside it (ChatIn.Pages). Nothing in their text is read
	// as a page.
	HistoryPagesApart = 2
)

const (
	currentPageMarker  = "\n\nCurrent page: "
	inboxPurposeMarker = "\nInbox purpose (context, not membership rules): "
)

// SplitPageContext separates the page a HistoryPagesInText message carries
// in its text from what the user typed before it. ok is false when the text
// ends in anything else, which then stays as written.
func SplitPageContext(text string, origins []string) (typed, href string, ok bool) {
	start := strings.LastIndex(text, currentPageMarker)
	if start < 0 {
		return text, "", false
	}
	href = text[start+len(currentPageMarker):]
	if page, purpose, found := strings.Cut(href, inboxPurposeMarker); found {
		// Exactly one JSON string, as the server wrote it: quotes at both
		// ends rule out null, numbers and surrounding whitespace, and a
		// valid JSON string holds no raw line breaks.
		if !strings.HasPrefix(purpose, `"`) || !strings.HasSuffix(purpose, `"`) || !json.Valid([]byte(purpose)) {
			return text, "", false
		}
		href = page
	}
	if MailContextKey(href, origins) == "" || strings.ContainsAny(href, "\n\r ") {
		return text, "", false
	}
	return text[:start], href, true
}

// messagePage is the page a transcript message was sent from, or empty.
func messagePage(ctx context.Context, prompt ChatItem) string {
	in, _ := ctx.Value(linksKey{}).(ChatIn)
	if prompt.ClientID == "" {
		return ""
	}
	return in.Pages[prompt.ClientID]
}

// userMessageText is a user message as the chat shows it: what the user
// typed, then the page it was sent from, which renders as a link.
func userMessageText(ctx context.Context, text, page string) string {
	if page != "" {
		return text + "\n\n" + page
	}
	in, _ := ctx.Value(linksKey{}).(ChatIn)
	if in.History >= HistoryPagesApart {
		return text
	}
	if typed, href, ok := SplitPageContext(text, in.LinkOrigins); ok {
		return typed + "\n\n" + href
	}
	return text
}

func queuedMessageText(item QueuedMessage) templ.Component {
	return withLinks(ChatIn{History: item.History, LinkOrigins: item.LinkOrigins, LinkPath: "/mail", LinkPaths: []string{"/files"}, LinkTarget: "[data-mail-content]", SelectedID: item.ThreadID}, templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if item.Links != nil {
			ctx = context.WithValue(ctx, linkLabelsKey{}, item.Links)
		}
		prompt := item.Prompt
		if item.Automated {
			prompt = strings.TrimPrefix(prompt, AutomatedNoticePrefix)
		}
		return chatLinkChips(ctx, uikit.LinkedText(userMessageText(ctx, prompt, item.Page), chatLinks(ctx))).Render(ctx, w)
	}))
}
