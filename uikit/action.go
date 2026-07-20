package uikit

import "github.com/a-h/templ"

type ActionIconProps struct {
	Attrs  templ.Attributes
	Danger bool
	Icon   templ.Component
	Label  string
}
