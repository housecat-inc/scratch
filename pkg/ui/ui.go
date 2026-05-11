package ui

import (
	"embed"
	"html/template"

	"github.com/cockroachdb/errors"
)

//go:embed templates
var templatesFS embed.FS

func ParseCode(funcs template.FuncMap) (*template.Template, error) {
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templatesFS, "templates/shared.html", "templates/code.html")
	if err != nil {
		return nil, errors.Wrap(err, "parse code templates")
	}
	return tmpl, nil
}

func ParseSessions() (*template.Template, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/shared.html", "templates/sessions.html")
	if err != nil {
		return nil, errors.Wrap(err, "parse sessions templates")
	}
	return tmpl, nil
}
