package utils

import "github.com/a-h/templ"

// OwnEvents marks an element whose events are its own. A component script
// that acts on events from anywhere inside its surface -- a collapsible's
// trigger, a row that opens on click -- leaves alone any event whose target
// sits inside an element carrying it. The element opts out this way rather
// than by stopping the event, so the event still reaches every other listener
// on the document. Not tied to any one component: every script that has a
// surface honours the same attribute, through eventOwner in
// ui/dom/housecat.ts.
func OwnEvents() templ.Attributes {
	return templ.Attributes{"data-own-events": true}
}
