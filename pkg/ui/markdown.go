package ui

import (
	"bytes"

	"github.com/a-h/templ"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

func Markdown(text string) templ.Component {
	var buf bytes.Buffer
	if err := markdown.Convert([]byte(text), &buf); err != nil {
		return templ.Raw(templ.EscapeString(text))
	}
	return templ.Raw(buf.String())
}
