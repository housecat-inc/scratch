// Package listbox is what the kit's listbox popups share: the top-layer
// positioner, the check on a selected option, and the class strings for the
// popup, its options and a field-like trigger. selectbox and combobox are built
// from it, so the two look and move alike without either copying the other.
// It is a building block for kit components, not for app code.
package listbox

import (
	"github.com/housecat-inc/scratch/ui/components/control"
	"github.com/housecat-inc/scratch/ui/utils"
)

type Align string

const (
	AlignCenter Align = "center"
	AlignEnd    Align = "end"
	AlignStart  Align = ""
)

// Name is the value data-align carries, which ui/floating reads.
func (a Align) Name() string {
	if a == AlignStart {
		return "start"
	}
	return string(a)
}

type Side string

const (
	SideBottom Side = ""
	SideTop    Side = "top"
)

// Name is the value data-side carries, which ui/floating reads.
func (s Side) Name() string {
	if s == SideBottom {
		return "bottom"
	}
	return string(s)
}

// FieldSize is a field trigger's height, on the same scale as the button
// family: Sm stands as tall as button.SizeSm and ButtonIcon's IconSizeMd, Xs as
// tall as IconSizeXs. Each size carries its own padding, gap and chevron. The
// start padding is the --listbox-trigger-inset custom property, set on the
// trigger itself, so FieldTriggerTextStart can undo exactly that padding.
type FieldSize string

const (
	FieldSizeSm FieldSize = ""
	FieldSizeXs FieldSize = "xs"
)

// Name is the value data-size carries.
func (s FieldSize) Name() string {
	if s == FieldSizeSm {
		return "sm"
	}
	return string(s)
}

func (s FieldSize) classes() string {
	if s == FieldSizeXs {
		return "h-6 gap-0.5 ps-(--listbox-trigger-inset) pr-1.5 [--listbox-trigger-inset:calc(var(--spacing)*2)]"
	}
	return "h-8 gap-1 ps-(--listbox-trigger-inset) pr-2 [--listbox-trigger-inset:calc(var(--spacing)*3)]"
}

// Chevron is the size, in pixels, of the trigger's chevron.
func (s FieldSize) Chevron() int {
	if s == FieldSizeXs {
		return 12
	}
	return 14
}

// FieldLook is how a field trigger draws, apart from how tall it is.
type FieldLook string

const (
	// FieldLookBox is a form field: control.Class's box, so a select or
	// combobox lines up with the inputs around it, ringed while its popup is
	// open as an input is while focused. It is the default.
	FieldLookBox FieldLook = ""
	// FieldLookPill is a muted pill that lights up on hover and while its
	// popup is open, for a picker set in a toolbar rather than a form, such as
	// the chat composer's model picker.
	FieldLookPill FieldLook = "pill"
)

// FieldTriggerClass is the field trigger's class for its size and look, with
// class merged last.
func FieldTriggerClass(size FieldSize, look FieldLook, class string) string {
	if look == FieldLookPill {
		return utils.TwMerge(
			"inline-flex shrink-0 cursor-pointer items-center squircle-pill bg-muted type-caption-100 text-foreground outline-none group/listbox-trigger",
			size.classes(),
			"hover:bg-accent hover:text-accent-foreground focus-visible:ring-2 focus-visible:ring-ring data-[popup-open]:bg-accent data-[popup-open]:text-accent-foreground",
			class,
		)
	}
	return utils.TwMerge(
		control.Class,
		control.Text,
		"inline-flex shrink-0 cursor-pointer items-center text-start group/listbox-trigger",
		size.classes(),
		"data-[popup-open]:border-ring data-[popup-open]:ring-[3px] data-[popup-open]:ring-ring/50",
		class,
	)
}

// FieldChevronClass colours the field trigger's chevron for its look: muted
// in a box, as an input's adornments are, and lit with a pill.
func FieldChevronClass(look FieldLook) string {
	if look == FieldLookPill {
		return "shrink-0 text-muted-foreground group-hover/listbox-trigger:text-accent-foreground group-data-[popup-open]/listbox-trigger:text-accent-foreground"
	}
	return "shrink-0 text-muted-foreground"
}

// FieldTriggerTextStart pulls a field trigger left by its own start padding, so
// the text inside it, rather than its rounded edge, lines up with a column of
// text beside it. It reads the size's own inset, so it is exact at every size.
const FieldTriggerTextStart = "-ms-(--listbox-trigger-inset)"

// PopupClass paints a listbox popup like the app's other menus. It is never
// narrower than its trigger: --anchor-width is the trigger's width, which
// listbox.ts writes on the positioner before placing it.
const PopupClass = "max-h-80 min-w-[var(--anchor-width)] overflow-y-auto rounded-md border bg-popover p-1 text-popover-foreground shadow-md outline-none"

// PopupSurfaceClass paints a popup like the app's other menus, without
// sizing or scrolling it: combobox's popup holds its input above the list and
// scrolls only the list, so the input never scrolls away.
const PopupSurfaceClass = "rounded-md border bg-popover p-1 text-popover-foreground shadow-md outline-none"

// PopupScrollClass is a listbox popup's height cap and scroll.
const PopupScrollClass = "max-h-80 overflow-y-auto"

// ItemClass is one option. Only the highlighted option is marked, and only
// script moves the mark: a hover rule of its own would light a second option
// whenever the pointer rested somewhere the keyboard had not put the cursor.
const ItemClass = "relative flex cursor-default flex-col rounded-sm py-1.5 pr-8 pl-2 outline-none select-none group/listbox-item data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground"

// ItemTextClass is an option's label.
const ItemTextClass = "type-body-200"

// ItemDescriptionClass is an option's secondary text under its label.
const ItemDescriptionClass = "type-caption-100 text-muted-foreground group-data-[highlighted]/listbox-item:text-accent-foreground"
