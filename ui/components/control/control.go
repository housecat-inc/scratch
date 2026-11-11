// Package control is what every form control shares, below the packages that
// draw them: Class, the box a control draws, and the Field it sits in, which
// field.Field puts in the context and field's controls, selectbox's trigger
// and combobox's take from it. It imports nothing of the kit, so listbox and
// field can both build on it.
package control

import (
	"context"

	"github.com/a-h/templ"

	"github.com/housecat-inc/scratch/ui/utils"
)

// Class is the box every control draws: 32px tall to sit beside a small
// button, bordered, with the focus ring the buttons use and a red border when
// aria-invalid. It sets no width; a control fills its field, and a trigger
// takes the width its caller gives it. Nor does it set a type style: a control
// adds Text, or MonoText for a value that is code.
const Class = "h-8 min-w-0 border border-input bg-background px-3 text-foreground squircle-lg squircle-amt-[1.5] outline-none transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"

// Text is a control's type: 16px below md, the size under which iOS Safari
// zooms the page in on a focused field, and 14px from md.
const Text = "type-body-300 md:type-body-200"

// MonoText is Text in the code face, at the same sizes, for a value that is
// code: a Gmail query, a cron expression.
const MonoText = "type-code-300 md:type-code-200"

type ctxKey int

const fieldKey ctxKey = iota

// Field is what a field hands the one control in it: the id to wear, unless
// the control has its own, what describes it, and whether it is invalid or
// required. Its methods are safe on nil, the Field of a control with none.
type Field struct {
	// DescribedBy is the ids of the field's help and error, space-separated.
	DescribedBy string
	// ID is the field's id, which a control without one of its own wears.
	ID      string
	Invalid bool
	// LabelID is the id of the field's label, or empty when it has none. A
	// control whose name would otherwise come from aria-label points
	// aria-labelledby at it instead.
	LabelID  string
	Required bool

	claimed bool
	// controlID is the id the claiming control wears, which the label points at.
	controlID string
	// owner is who claimed the Field through ClaimAs.
	owner string
}

// With hands f to the control under ctx.
func With(ctx context.Context, f *Field) context.Context {
	return context.WithValue(ctx, fieldKey, f)
}

// Claim is the Field the control under ctx sits in, or nil. Only the first
// control to ask gets it: a field labels one control, and a second one in it
// -- a unit beside a number -- is on its own rather than sharing the first's
// id, label and error.
func Claim(ctx context.Context) *Field {
	return ClaimAs(ctx, "")
}

// ClaimAs is Claim for a control that may ask more than once -- a trigger
// whose attributes every part drawing it reads -- naming itself by owner:
// the owner that claimed the Field gets it back each time it asks, and
// anyone else gets nil.
func ClaimAs(ctx context.Context, owner string) *Field {
	f, _ := ctx.Value(fieldKey).(*Field)
	if f == nil {
		return nil
	}
	if f.claimed {
		if owner != "" && f.owner == owner {
			return f
		}
		return nil
	}
	f.claimed, f.owner = true, owner
	return f
}

// Claimable reports whether ClaimAs(owner) would hand f over, without
// claiming it: whether the control about to ask would wear f.
func (f *Field) Claimable(owner string) bool {
	return f != nil && (!f.claimed || owner != "" && f.owner == owner)
}

// RootOf is the id and required state of a control's root, before its
// trigger claims the Field under ctx. While the Field is unclaimed, the
// trigger about to claim it is the field's control: the root takes the
// field's id with suffix when own is empty, and is required when the field
// is. A second control in the field is on its own, with own and required
// alone. The id is empty when own is and the root takes none from a field.
func RootOf(ctx context.Context, own, suffix string, required bool) (string, bool) {
	f := Peek(ctx)
	mine := f.Claimable("")
	if own == "" && mine {
		own = f.ID + suffix
	}
	return own, required || mine && f.IsRequired()
}

// Peek is the Field under ctx, claimed or not, for a part of a control
// that only reads it -- a popup labelled by the field's label -- and must
// not take it from the part that wears its id.
func Peek(ctx context.Context) *Field {
	f, _ := ctx.Value(fieldKey).(*Field)
	return f
}

// Take is the id the control wears: its own, or the field's when it has
// none. The field's label points at whichever it is.
func (f *Field) Take(own string) string {
	if f == nil {
		return own
	}
	f.controlID = utils.IfElse(own != "", own, f.ID)
	return f.controlID
}

// ControlID is the id the label points at: the claiming control's, or the
// field's own while no control has claimed it.
func (f *Field) ControlID() string {
	if f == nil {
		return ""
	}
	return utils.IfElse(f.controlID != "", f.controlID, f.ID)
}

// IsRequired reports whether the field requires its control.
func (f *Field) IsRequired() bool {
	return f != nil && f.Required
}

// Attributes are the control's aria-describedby and aria-invalid.
func (f *Field) Attributes() templ.Attributes {
	attrs := templ.Attributes{}
	if f == nil {
		return attrs
	}
	if f.DescribedBy != "" {
		attrs["aria-describedby"] = f.DescribedBy
	}
	if f.Invalid {
		attrs["aria-invalid"] = "true"
	}
	return attrs
}

// IDOr is id, or a random one when it is empty. It is not utils.IfElse,
// which would draw the random id even when it goes unused.
func IDOr(id string) string {
	if id != "" {
		return id
	}
	return utils.RandomID()
}

func Disabled(own bool, attributes templ.Attributes) bool {
	if own {
		return true
	}
	value, present := attributes["disabled"]
	if disabled, ok := value.(bool); ok {
		return disabled
	}
	return present
}
