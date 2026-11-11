// Package combobox is a searchable select, single or multiple, after Base
// UI's Combobox with its input inside the popup: a trigger that opens a
// dialog holding a text input and the listbox it filters. The trigger is the
// form control; the input is a combobox of its own that controls the list.
// Options take the cursor by aria-activedescendant while focus stays in the
// input.
//
// It shares its positioner, check, option and field-trigger looks with
// selectbox through ui/components/listbox, so the two look and move alike.
// Behaviour is combobox.ts in the ui.js bundle: filtering, the keys, toggling
// or picking, the before and after change events, the create option, and the
// hidden inputs a form submits.
package combobox

import (
	"context"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/housecat-inc/scratch/ui/components/control"
	"github.com/housecat-inc/scratch/ui/components/listbox"
)

// Entry is one option's label and value, which Root takes so TriggerField can
// name the chosen ones without the list.
type Entry struct {
	Label string
	Value string
}

type Align = listbox.Align

const (
	AlignCenter = listbox.AlignCenter
	AlignEnd    = listbox.AlignEnd
	AlignStart  = listbox.AlignStart
)

type Side = listbox.Side

const (
	SideBottom = listbox.SideBottom
	SideTop    = listbox.SideTop
)

// Size is TriggerField's height; see listbox.FieldSize.
type Size = listbox.FieldSize

const (
	SizeSm = listbox.FieldSizeSm
	SizeXs = listbox.FieldSizeXs
)

// Look is how a field trigger draws; see listbox.FieldLook.
type Look = listbox.FieldLook

const (
	LookBox  = listbox.FieldLookBox
	LookPill = listbox.FieldLookPill
)

type ctxKey int

const rootKey ctxKey = iota

type root struct {
	disabled *bool
	id       string
	items    []Entry
	label    string
	multiple bool
	required bool
	value    []string
}

// newRoot is the Root's context; a single combobox keeps only the first of
// Value.
func newRoot(ctx context.Context, props RootProps) root {
	id, required := rootOf(ctx, props)
	value := props.Value
	if !props.Multiple && len(value) > 1 {
		value = value[:1]
	}
	return root{
		disabled: new(bool),
		id:       id,
		items:    props.Items,
		label:    props.Label,
		multiple: props.Multiple,
		required: required,
		value:    value,
	}
}

// rootOf is the Root's id and whether it is required, as control.RootOf
// has them; an id still unset is random.
func rootOf(ctx context.Context, props RootProps) (string, bool) {
	id, required := control.RootOf(ctx, props.ID, "-combobox", props.Required)
	return control.IDOr(id), required
}

func rootFrom(ctx context.Context) root {
	r, _ := ctx.Value(rootKey).(root)
	return r
}

func (r root) triggerID() string    { return r.id + "-trigger" }
func (r root) positionerID() string { return r.id + "-positioner" }
func (r root) popupID() string      { return r.id + "-popup" }
func (r root) inputID() string      { return r.id + "-input" }
func (r root) listID() string       { return r.id + "-list" }
func (r root) createID() string     { return r.id + "-create" }

func (r root) groupLabelID(value string) string { return r.id + "-group-" + value + "-label" }

// itemID names an option so the input can point at it with
// aria-activedescendant. Values are unique within a root.
func (r root) itemID(value string) string { return r.id + "-item-" + value }

func (r root) selected(value string) bool { return slices.Contains(r.value, value) }

// valueText is what TriggerField shows: the chosen labels in Items order,
// joined, or placeholder when none is chosen.
func (r root) valueText(placeholder string) string {
	var chosen []string
	for _, e := range r.items {
		if r.selected(e.Value) {
			chosen = append(chosen, e.Label)
		}
	}
	if len(chosen) == 0 {
		return placeholder
	}
	return strings.Join(chosen, ", ")
}

// TriggerAttributes is the attribute set that makes an element the
// combobox's trigger: the combobox role naming the dialog it opens, its id,
// its slot, and the popovertarget the browser opens the popup from. Trigger,
// TriggerField and TriggerIcon all carry exactly these. In a field.Field the
// trigger is named by the Field's label, which points at its id, and is
// described by the Field's help and error, however many times it is asked.
func TriggerAttributes(ctx context.Context) templ.Attributes {
	r := rootFrom(ctx)
	f := control.ClaimAs(ctx, r.triggerID())
	attrs := templ.Attributes{
		"aria-controls": r.popupID(),
		"aria-expanded": "false",
		"aria-haspopup": "dialog",
		"data-slot":     "combobox-trigger",
		"id":            f.Take(r.triggerID()),
		"popovertarget": r.positionerID(),
		"role":          "combobox",
	}
	if f != nil && f.LabelID != "" {
		attrs["aria-labelledby"] = f.LabelID
	} else {
		attrs["aria-label"] = r.label
	}
	if r.required || f.IsRequired() {
		attrs["aria-required"] = "true"
	}
	for k, v := range f.Attributes() {
		attrs[k] = v
	}
	return attrs
}

// Indicator is how an option shows that it is selected.
type Indicator string

const (
	// IndicatorCheck is the default: a check on the right of a selected
	// option.
	IndicatorCheck Indicator = ""
	// IndicatorCheckbox is a checkbox on the left of every option, ticked
	// when it is selected, for a multiple combobox that reads as a list of
	// toggles.
	IndicatorCheckbox Indicator = "checkbox"
	// IndicatorRing is for an option that is a tile of its own, in a Grid
	// combobox: the children are drawn in place of the label text, which
	// still names the option, and a selected option has a ring round it, apart
	// from the highlight's background.
	IndicatorRing Indicator = "ring"
)

// ItemRingClass turns an option into a centred tile whose selection is an
// inset ring, which the highlight's background and a ring can show together.
const ItemRingClass = "items-center justify-center rounded-md p-0 data-[selected]:ring-2 data-[selected]:ring-primary data-[selected]:ring-inset"

// GroupClass hides a Group while no option in it shows.
const GroupClass = "[&:not(:has([data-slot=combobox-item]:not([hidden])))]:hidden"

// GroupLabelClass is a Group's heading, the look of the menus' group labels.
const GroupLabelClass = "px-2 pt-2 pb-1 type-caption-100-medium text-muted-foreground"

// EmptyClass is Empty's look.
const EmptyClass = "px-2 py-1.5 type-body-200 text-muted-foreground"

// ItemCheckboxRowClass makes room for the checkbox on the left of an option
// and gives back the room the right-hand check would take.
const ItemCheckboxRowClass = "pr-2 pl-8"

// ItemCheckIconClass is the check inside the checkbox: invisible until the
// option is selected, the way listbox.ItemIndicator hides its check.
const ItemCheckIconClass = "invisible group-data-[selected]/listbox-item:visible"

// ItemCheckboxClass is the checkbox drawn on the left of an option: an empty
// bordered box, filled with a check when the option is selected. On the
// highlighted option it is drawn in the highlight's foreground, since the
// highlight and a ticked box share a colour. The check inside is hidden by
// ItemCheckIconClass, not by colour, so forced-colors mode cannot reveal it.
const ItemCheckboxClass = "absolute top-2 left-2 flex size-4 items-center justify-center rounded border border-muted-foreground " +
	"group-data-[selected]/listbox-item:bg-primary group-data-[selected]/listbox-item:text-primary-foreground " +
	"group-[[data-selected]:not([data-highlighted])]/listbox-item:border-primary " +
	"group-data-[highlighted]/listbox-item:border-accent-foreground"

func triggerDisabled(ctx context.Context, attributes templ.Attributes, own bool) {
	r := rootFrom(ctx)
	if r.disabled != nil {
		*r.disabled = *r.disabled || control.Disabled(own, attributes)
	}
}
