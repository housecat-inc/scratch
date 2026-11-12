package codex

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/housecat-inc/scratch/ui/components/linkchip"
	"golang.org/x/net/html"
)

var gmailActionLabel = regexp.MustCompile(`(?i)^(?:(?:open|view|read)(?: (?:the |this )?(?:email|message|thread|conversation|link))? (?:in |on )?)?gmail$`)

type (
	// LinkLabel is how a chat link reads: its label and, after it, the
	// context that tells it apart, such as the account.
	LinkLabel struct {
		Context string `json:"context"`
		Label   string `json:"label"`
	}
	LinkResolver    func(context.Context, *url.URL) LinkLabel
	linkResolverKey struct{}
	linkLabelsKey   struct{}
)

// LinkPreviewOut is /codex/link-preview's answer: the link's label, and the
// mail context key (MailContextKey) the page context compares with the one
// the chat last sent.
type LinkPreviewOut struct {
	LinkLabel
	Key string `json:"key"`
}

func WithLinkResolver(ctx context.Context, resolve LinkResolver) context.Context {
	type entry struct {
		label LinkLabel
		until time.Time
	}
	var mu sync.Mutex
	cache := map[string]entry{}
	return context.WithValue(ctx, linkResolverKey{}, LinkResolver(func(ctx context.Context, u *url.URL) LinkLabel {
		mu.Lock()
		defer mu.Unlock()
		key := u.String()
		if cached, ok := cache[key]; ok && time.Now().Before(cached.until) {
			return cached.label
		}
		label := resolve(ctx, u)
		if ctx.Err() != nil {
			return label
		}
		if len(cache) >= 256 {
			for old := range cache {
				delete(cache, old)
				break
			}
		}
		duration := 5 * time.Minute
		if label.Label == "" {
			duration = 30 * time.Second
		}
		cache[key] = entry{label: label, until: time.Now().Add(duration)}
		return label
	}))
}

func chatLinkChips(ctx context.Context, content templ.Component) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		var source bytes.Buffer
		if err := content.Render(ctx, &source); err != nil {
			return err
		}
		tokenizer := html.NewTokenizer(&source)
		for {
			kind := tokenizer.Next()
			if kind == html.ErrorToken {
				if tokenizer.Err() == io.EOF {
					return nil
				}
				return tokenizer.Err()
			}
			raw := string(tokenizer.Raw())
			if kind != html.StartTagToken {
				if _, err := io.WriteString(w, raw); err != nil {
					return err
				}
				continue
			}
			token := tokenizer.Token()
			if token.Data != "a" {
				if _, err := io.WriteString(w, raw); err != nil {
					return err
				}
				continue
			}
			attrs := templ.Attributes{}
			href := ""
			for _, a := range token.Attr {
				attrs[a.Key] = a.Val
				if a.Key == "href" {
					href = a.Val
				}
			}
			var inner, label strings.Builder
			for {
				k := tokenizer.Next()
				if k == html.ErrorToken {
					return tokenizer.Err()
				}
				r := string(tokenizer.Raw())
				t := tokenizer.Token()
				if k == html.EndTagToken && t.Data == "a" {
					break
				}
				inner.WriteString(r)
				if k == html.TextToken {
					label.WriteString(t.Data)
				}
			}
			original := strings.TrimSpace(label.String())
			innerHTML := inner.String()
			in, _ := ctx.Value(linksKey{}).(ChatIn)
			if u, err := url.Parse(href); err == nil && strings.HasPrefix(u.Path, "/mail/") && in.linkAttributes(href)["href"] != "" && gmailActionLabel.MatchString(original) {
				innerHTML = "Open email"
			}
			if notice, ok := in.draftNotice(href); ok {
				if err := draftStatus(notice).Render(ctx, w); err != nil {
					return err
				}
				continue
			}
			if u, err := url.Parse(href); err == nil && u.Path == "/mail" && u.Query().Get("draft") != "" && in.linkAttributes(href)["href"] != "" {
				delete(attrs, "title")
				chipLabel := original
				if chipLabel == "" || chipLabel == href || in.linkAttributes(original)["href"] == href {
					chipLabel = "Review draft"
				}
				if err := linkchip.Chip(linkchip.In{Attributes: attrs, Label: chipLabel, URL: href}).Render(ctx, w); err != nil {
					return err
				}
				continue
			}
			if externalLinkLabel(href, original) != "" && in.linkAttributes(href)["target"] == "_blank" {
				delete(attrs, "title")
				if err := linkchip.Chip(linkchip.In{Attributes: attrs, Label: externalLinkLabel(href, original), URL: href}).Render(ctx, w); err != nil {
					return err
				}
				continue
			}
			updates := chatLinks(ctx)(original)
			// URL labels become chips; authored link text keeps its formatting.
			if original != "" && href != "" && updates["href"] == href {
				u, err := url.Parse(normalizedLink(ctx, original))
				if err == nil {
					preview := fallbackLinkLabel(u)
					cache, _ := ctx.Value(linkLabelsKey{}).(map[string]LinkLabel)
					if cached, ok := cache[original]; ok {
						preview = cached
					} else {
						if resolve, ok := ctx.Value(linkResolverKey{}).(LinkResolver); ok && len(cache) < 50 {
							if resolved := resolve(ctx, u); resolved.Label != "" {
								preview = resolved
							}
						}
						if cache != nil {
							cache[original] = preview
						}
					}
					delete(attrs, "title")
					if err := linkchip.Chip(linkchip.In{Attributes: attrs, Label: preview.Label, Context: preview.Context, URL: original}).Render(ctx, w); err != nil {
						return err
					}
					continue
				}
			}
			if _, err := io.WriteString(w, raw+innerHTML+"</a>"); err != nil {
				return err
			}
		}
	})
}

func fallbackLinkLabel(u *url.URL) LinkLabel {
	out := LinkLabel{Label: "Mail", Context: u.Query().Get("account")}
	if strings.Trim(u.Path, "/") != "mail" {
		out.Label = "Email thread"
		return out
	}
	if u.Query().Has("label") {
		out.Label = "Label collection"
		if provider := u.Query().Get("provider"); provider == "" || provider == "housecat" {
			out.Context = ""
		}
		return out
	}
	labels := map[string]string{"INBOX": "Inbox", "IMPORTANT": "Correspondence", "FYI": "Notifications", "OTHER": "Broadcasts", "COLD_OUTREACH": "Cold outreach", "ACTION_REQUIRED": "Action required", "GMAIL_IMPORTANT": "Important", "SENT": "Sent", "DRAFT": "Drafts", "STARRED": "Starred", "TRASH": "Trash", "SPAM": "Spam", "ALL": "All mail"}
	if label := labels[u.Query().Get("view")]; label != "" {
		out.Label = label
	}
	if u.Query().Get("q") != "" {
		out.Label = "Mail search"
	}
	return out
}

func linkRenderContext(ctx context.Context) (context.Context, context.CancelFunc) {
	lookupCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	if normalize, ok := ctx.Value(linkNormalizerKey{}).(func(string) string); ok {
		var mu sync.Mutex
		cache := map[string]string{}
		ctx = WithLinkNormalizer(ctx, func(href string) string {
			mu.Lock()
			defer mu.Unlock()
			if normalized, ok := cache[href]; ok {
				return normalized
			}
			normalized := normalize(href)
			if len(cache) < 256 {
				cache[href] = normalized
			}
			return normalized
		})
	}
	if resolve, ok := ctx.Value(linkResolverKey{}).(LinkResolver); ok {
		ctx = context.WithValue(ctx, linkResolverKey{}, LinkResolver(func(_ context.Context, u *url.URL) LinkLabel {
			if lookupCtx.Err() != nil {
				return LinkLabel{}
			}
			return resolve(lookupCtx, u)
		}))
	}
	return context.WithValue(ctx, linkLabelsKey{}, map[string]LinkLabel{}), cancel
}

func LinkPreview(ctx context.Context, href string, origins []string) LinkLabel {
	href = normalizedLink(ctx, href)
	if strings.HasPrefix(strings.ToLower(href), "www.") {
		href = "https://" + href
	}
	u, err := url.Parse(href)
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https" && !(u.Scheme == "" && u.Host == "" && strings.HasPrefix(u.Path, "/"))) {
		return LinkLabel{}
	}
	in := ChatIn{LinkOrigins: origins, LinkPath: "/mail"}
	if in.linkAttributes(href)["href"] == "" {
		return LinkLabel{Label: u.Hostname()}
	}
	ctx, cancel := linkRenderContext(ctx)
	defer cancel()
	if resolve, ok := ctx.Value(linkResolverKey{}).(LinkResolver); ok {
		if label := resolve(ctx, u); label.Label != "" {
			return label
		}
	}
	return fallbackLinkLabel(u)
}

func externalLinkLabel(href, label string) string {
	candidate := label
	if strings.HasPrefix(strings.ToLower(candidate), "www.") {
		candidate = "https://" + candidate
	}
	if href != candidate {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.Hostname()
}
