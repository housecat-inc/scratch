package wire

// LabelVisibilityChange is one of the visibilities the sidebar's label menu
// saves in one post to /mail/labels/visibility (its changes field, a list):
// the label, the field (list or message) and the value Gmail takes for it.
type LabelVisibilityChange struct {
	Field string `json:"field"`
	Label string `json:"label"`
	Value string `json:"value"`
}
