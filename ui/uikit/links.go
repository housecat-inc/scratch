package uikit

import (
	"bytes"
	"html"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/a-h/templ"
	nethtml "golang.org/x/net/html"
)

type LinkAttributes func(string) map[string]string

var textURL = regexp.MustCompile("(?i)\\b(?:https?://|www\\.)[^\\s<>\"'`]+")

func LinkedText(text string, attributes LinkAttributes) templ.Component {
	var out strings.Builder
	offset := 0
	for _, match := range textURL.FindAllStringIndex(text, -1) {
		label := strings.TrimRight(text[match[0]:match[1]], ".,!?;:")
		for _, pair := range [][2]string{{"(", ")"}, {"[", "]"}, {"{", "}"}} {
			for strings.HasSuffix(label, pair[1]) && strings.Count(label, pair[1]) > strings.Count(label, pair[0]) {
				label = strings.TrimRight(strings.TrimSuffix(label, pair[1]), ".,!?;:")
			}
		}
		href := label
		if strings.HasPrefix(strings.ToLower(href), "www.") {
			href = "https://" + href
		}
		u, err := url.Parse(href)
		if err != nil || u.Hostname() == "" {
			continue
		}
		out.WriteString(html.EscapeString(text[offset:match[0]]))
		out.WriteString(`<a href="` + html.EscapeString(href) + `">` + html.EscapeString(label) + `</a>`)
		offset = match[0] + len(label)
	}
	out.WriteString(html.EscapeString(text[offset:]))
	return templ.Raw(string(linkAttributes([]byte(out.String()), attributes)))
}

func linkAttributes(source []byte, attributes LinkAttributes) []byte {
	if attributes == nil {
		return source
	}
	var out bytes.Buffer
	tokenizer := nethtml.NewTokenizer(bytes.NewReader(source))
	for {
		kind := tokenizer.Next()
		if kind == nethtml.ErrorToken {
			if tokenizer.Err() != io.EOF {
				return source
			}
			return out.Bytes()
		}
		raw := append([]byte(nil), tokenizer.Raw()...)
		if kind != nethtml.StartTagToken {
			out.Write(raw)
			continue
		}
		token := tokenizer.Token()
		if token.Data != "a" {
			out.Write(raw)
			continue
		}
		href := ""
		for _, attr := range token.Attr {
			if attr.Key == "href" {
				href = attr.Val
				break
			}
		}
		if href == "" {
			out.Write(raw)
			continue
		}
		updates := attributes(href)
		keys := make([]string, 0, len(updates))
		for key := range updates {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			token.Attr = slices.DeleteFunc(token.Attr, func(attr nethtml.Attribute) bool { return attr.Key == key })
			token.Attr = append(token.Attr, nethtml.Attribute{Key: key, Val: updates[key]})
		}
		out.WriteString(token.String())
	}
}
