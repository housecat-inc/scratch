// Package css embeds the shared stylesheets for Go code that reads their
// tokens, such as the Foundations stories in ui/storybook.
package css

import _ "embed"

//go:embed theme.css
var Theme string
