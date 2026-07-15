package ui

import (
	_ "embed"

	"github.com/a-h/templ"
)

//go:embed static/templui-themes.css
var templuiThemesCSS string

//go:embed static/templui-tailwind.css
var templuiTailwindCSS string

//go:embed static/inbox.css
var inboxCSS string

//go:embed static/datatable.css
var dataTableCSS string

//go:embed static/sw.js
var serviceWorkerJS []byte

func DataTableCSS() templ.Component {
	return templ.Raw(`<style>` + dataTableCSS + `</style>`)
}

func InboxCSS() templ.Component {
	return templ.Raw(`<style>` + inboxCSS + `</style>`)
}

func TempluiCSS() templ.Component {
	return templ.Raw(`<style>` + templuiThemesCSS + `</style>` +
		`<style type="text/tailwindcss">` + templuiTailwindCSS + `</style>`)
}

func ServiceWorkerJS() []byte { return serviceWorkerJS }
