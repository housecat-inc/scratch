package uikit

import (
	"bytes"
	"regexp"

	"github.com/a-h/templ"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

var markdownPolicy = func() *bluemonday.Policy {
	policy := bluemonday.NewPolicy()
	policy.AllowElements("a", "blockquote", "br", "code", "del", "em", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "li", "ol", "p", "pre", "strong", "table", "tbody", "td", "th", "thead", "tr", "ul")
	policy.AllowAttrs("align").OnElements("td", "th")
	policy.AllowAttrs("class").OnElements("code")
	policy.AllowAttrs("href", "title").OnElements("a")
	policy.AllowAttrs("checked", "disabled").OnElements("input")
	policy.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	policy.AllowElements("input")
	policy.AllowStandardURLs()
	policy.RequireNoFollowOnLinks(true)
	policy.RequireNoReferrerOnLinks(true)
	return policy
}()

func Markdown(text string) templ.Component {
	return MarkdownWithLinks(text, nil)
}

func MarkdownWithLinks(text string, attributes LinkAttributes) templ.Component {
	var buf bytes.Buffer
	if err := markdown.Convert([]byte(text), &buf); err != nil {
		return templ.Raw(templ.EscapeString(text))
	}
	return templ.Raw(string(linkAttributes(markdownPolicy.SanitizeBytes(buf.Bytes()), attributes)))
}
