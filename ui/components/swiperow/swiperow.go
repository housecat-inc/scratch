// Package swiperow is a list row that slides left under a finger to uncover a
// tray of actions at its right end, on a touch screen: no hover and a coarse
// pointer. A pointer screen reveals the same actions on hover instead, so every
// class here is gated on the touch variant (ui/css/theme.css) and a mouse never
// sees a tray.
//
// The row is not a wrapper: a caller marks its own row element with Attributes
// and RowClass, and its own action cluster with TrayClass and
// TrayAttributes. ui/dom/swipe.ts drives both from there, on pans ui/dom/pan.ts
// hands it: data-swipe on the row says closed, open or armed, and --swipe-x is
// how far the row's content has slid.
package swiperow

import (
	"strconv"

	"github.com/a-h/templ"
)

// The thresholds, written on the row as data attributes that swipe.ts reads
// with intAttr when a drag starts, so each is a whole percent. A call site
// can tune one by prop, and a device all of them with ?swipe= (swipe.ts),
// without a deploy.
const (
	// CommitAt is the percent of the viewport -- or of the row, where the
	// row is narrower -- a drag has to pass to arm the default action. The
	// drag must also pass the complete tray by CommitPastPx. Flicks open the
	// tray instead of committing.
	CommitAt = 50
)

// Props overrides the thresholds for one call site. A field of zero or less
// keeps the default, so a row never carries a threshold swipe.ts would read
// as one that is always passed.
type Props struct {
	CommitAt int
	// CommitPastPx is the extra pull beyond the fully revealed dragging tray.
	// Zero uses one button width.
	CommitPastPx int
	// OpenAt is the percent of the tray's width a released drag has to pass
	// for the row to snap open, as data-swipe-open-at. Zero leaves swipe.ts's
	// own, where the first action and its release cue are uncovered, and the
	// row carries no attribute.
	OpenAt int
	// RevealAtPx is the minimum available button space before scaling starts.
	// Zero uses 8px.
	RevealAtPx int
	// SlopPx is the least a pan on the row travels before ui/dom/pan.ts
	// decides it, as data-pan-slop-px. A pan that could also be another
	// owner's waits for the larger of their slops. Zero leaves pan.ts's own,
	// and the row carries no attribute.
	SlopPx int
}

// Attributes marks a row as swipeable: data-swipe starts closed, and the
// thresholds are data attributes. A caller's attribute of the same name
// replaces one of these.
func Attributes(p Props) templ.Attributes {
	out := templ.Attributes{
		"data-swipe":           "closed",
		"data-swipe-commit-at": strconv.Itoa(or(p.CommitAt, CommitAt)),
	}
	if p.OpenAt > 0 {
		out["data-swipe-open-at"] = strconv.Itoa(p.OpenAt)
	}
	if p.CommitPastPx > 0 {
		out["data-swipe-commit-past-px"] = strconv.Itoa(p.CommitPastPx)
	}
	if p.RevealAtPx > 0 {
		out["data-swipe-reveal-at-px"] = strconv.Itoa(p.RevealAtPx)
	}
	if p.SlopPx > 0 {
		out["data-pan-slop-px"] = strconv.Itoa(p.SlopPx)
	}
	return out
}

// TrayAttributes marks the row's tray, the child swipe.ts measures and leaves
// in place while the rest of the row slides. The action it runs on a full
// swipe is the tray's button carrying data-swipe-commit; a tray without one
// only opens.
func TrayAttributes() templ.Attributes {
	return templ.Attributes{"data-swipe-part": "tray"}
}

func or(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}

// RowClass goes on the row. It clips the content sliding out of it, leaves
// vertical panning to the browser so a sideways pan reaches ui/dom/pan.ts as
// pointer events, and slides the content by --swipe-x through the
// swipe-content variant (ui/css/theme.css), which reaches through the
// display:contents wrappers a row's link and tooltips put around its boxes.
//
// touch-action is declared as plain pan-y, which every touch browser accepts.
// WebKit does not know pinch-zoom, and a touch-action it cannot parse is
// dropped for auto, which would let iOS Safari claim every pan and cancel the
// swipe; so pan-y pinch-zoom, which keeps a pinch that starts on a row
// zooming, is layered on only where @supports says the browser takes it. The
// arbitrary properties keep both values literal: Tailwind's touch-pan-y and
// touch-pinch-zoom compose them through variables, which would fail at
// computed-value time rather than parse time and dodge the @supports check.
//
// The content's transition is translate only, which outranks the
// transition-all a star or ButtonIcon in the row brings, so those lose their
// own easing on a touch screen; nothing there animates on touch that matters.
//
// The slide animates only when the finger lets go: data-swipe-dragging is set
// for the length of a drag, so the content tracks the finger, and reduced
// motion snaps it.
//
// A long press on the row opens its action card (ui/dom/pan.ts's hold), its
// links included, so the browser's own long press is turned off: iOS's link
// preview with the -webkit-touch-callout the links inherit, and text
// selection, which would otherwise start on the row before the hold or on the
// card that opens under the finger.
//
// group/swipe is what the tray reads the row's state through.
const RowClass = "group/swipe touch:[-webkit-touch-callout:none] touch:select-none touch:bg-transparent touch:data-[read-state=unread]:bg-transparent touch:overflow-hidden touch:[touch-action:pan-y] touch:supports-[touch-action:pan-y_pinch-zoom]:[touch-action:pan-y_pinch-zoom]" +
	" swipe-content:relative swipe-content:z-1 swipe-content:translate-x-[var(--swipe-x,0px)] swipe-content:transition-[translate] swipe-content:duration-250 swipe-content:ease-[cubic-bezier(0.2,0.8,0.2,1)]" +
	" data-[swipe-dragging]:swipe-content:transition-none motion-reduce:swipe-content:transition-none" +
	" touch:before:absolute touch:before:inset-0 touch:before:squircle-[0px] touch:before:bg-transparent touch:data-[swipe-active]:before:squircle-lg touch:data-[swipe-active]:before:bg-foreground/5 touch:before:translate-x-[var(--swipe-x,0px)] touch:before:transition-[translate,background-color,border-radius] touch:before:duration-250 touch:before:ease-[cubic-bezier(0.2,0.8,0.2,1)] touch:data-[swipe-dragging]:before:transition-[background-color,border-radius] touch:motion-reduce:before:transition-none"

// A dragging tray reserves space for its visible release cue; idle trays
// reclaim it for their actions.
const TrayClass = "group/swipe-tray touch:[--swipe-chevron-direction:1] touch:data-[swipe-side=right]:[--swipe-chevron-direction:-1] touch:absolute touch:inset-y-0 touch:right-0 touch:z-[2] touch:flex touch:items-stretch touch:justify-end touch:[--swipe-chevron-space:0px] touch:group-data-[swipe-dragging]/swipe:[--swipe-chevron-space:calc(var(--swipe-chevron-reserve,0)*1px)] touch:gap-0 touch:pl-[var(--swipe-leading,var(--swipe-chevron-space,0px))] touch:pr-0 touch:opacity-100 touch:pointer-events-auto" +
	" touch:[--swipe-distance:max(0px,calc(var(--swipe-x,0px)*-1))] touch:min-w-[var(--swipe-distance)] touch:[clip-path:inset(0_0_0_max(0px,calc(100%-var(--swipe-distance))))]" +
	" touch:data-[swipe-side=right]:left-0 touch:data-[swipe-side=right]:right-auto touch:data-[swipe-side=right]:justify-start touch:data-[swipe-side=right]:pl-0 touch:data-[swipe-side=right]:pr-[var(--swipe-leading,var(--swipe-chevron-space,0px))] touch:data-[swipe-side=right]:[--swipe-distance:max(0px,var(--swipe-x,0px))] touch:data-[swipe-side=right]:[clip-path:inset(0_max(0px,calc(100%-var(--swipe-distance)))_0_0)]" +
	" touch:group-data-[swipe=armed]/swipe:[--swipe-leading:0px] touch:transition-[clip-path,min-width,padding-left,padding-right] touch:duration-250 touch:ease-[cubic-bezier(0.2,0.8,0.2,1)] touch:group-data-[swipe-dragging]/swipe:transition-none touch:motion-reduce:transition-none"

const ActionClass = "contents group/swipe-action touch:has-[[data-swipe-overflow]]:hidden! touch:group-data-[presentation=swipe]/swipe-tray:relative touch:group-data-[presentation=swipe]/swipe-tray:grid touch:group-data-[presentation=swipe]/swipe-tray:grid-cols-1 touch:group-data-[presentation=swipe]/swipe-tray:content-center touch:group-data-[presentation=swipe]/swipe-tray:gap-y-1.5 touch:group-data-[presentation=swipe]/swipe-tray:w-[var(--swipe-slot-width,calc(56px+var(--swipe-margin,var(--entry-list-p-l,12px))*2))] touch:group-data-[presentation=swipe]/swipe-tray:flex-none touch:group-data-[presentation=swipe]/swipe-tray:items-center touch:group-data-[presentation=swipe]/swipe-tray:h-full touch:group-data-[presentation=swipe]/swipe-tray:py-2 touch:group-data-[presentation=swipe]/swipe-tray:pl-[var(--swipe-pad-left,var(--swipe-margin,var(--entry-list-p-l,12px)))] touch:group-data-[presentation=swipe]/swipe-tray:pr-[var(--swipe-pad-right,var(--swipe-margin,var(--entry-list-p-l,12px)))] touch:group-data-[presentation=swipe]/swipe-tray:origin-center touch:group-data-[presentation=swipe]/swipe-tray:scale-[var(--swipe-scale,0)] touch:group-data-[presentation=swipe]/swipe-tray:opacity-[var(--swipe-opacity,0)] touch:group-data-[presentation=swipe]/swipe-tray:translate-x-[var(--swipe-circle-shift,0px)] touch:group-data-[presentation=swipe]/swipe-tray:transition-[scale,opacity,translate] touch:group-data-[presentation=swipe]/swipe-tray:duration-250 touch:group-data-[swipe-dragging]/swipe:transition-none touch:motion-reduce:transition-none" +
	" touch:group-data-[presentation=swipe]/swipe-tray:has-[[data-swipe-commit]]:order-last touch:group-data-[swipe-side=right]/swipe-tray:has-[[data-swipe-commit]]:order-first" +
	" touch:group-data-[swipe=armed]/swipe:not-has-[[data-swipe-commit]]:hidden touch:group-data-[swipe=armed]/swipe:has-[[data-swipe-commit]]:w-[var(--swipe-distance)] touch:group-data-[swipe=armed]/swipe:has-[[data-swipe-commit]]:px-[var(--swipe-margin,var(--entry-list-p-l,12px))]"

// CaptionClass is an action's caption under its circle, shown only in a
// touch screen's swipe tray. It holds one caption per state the action can
// show (Captions), and hides every one but the current.
const CaptionClass = "hidden pointer-events-none touch:group-data-[presentation=swipe]/swipe-tray:block touch:group-data-[presentation=swipe]/swipe-tray:row-start-2 touch:group-data-[presentation=swipe]/swipe-tray:col-start-1 touch:group-data-[presentation=swipe]/swipe-tray:cap-trim touch:group-data-[presentation=swipe]/swipe-tray:mx-[calc(min(var(--swipe-pad-left,0px),var(--swipe-pad-right,0px))*-1)] touch:group-data-[presentation=swipe]/swipe-tray:whitespace-normal touch:group-data-[presentation=swipe]/swipe-tray:break-words touch:group-data-[presentation=swipe]/swipe-tray:text-center touch:group-data-[presentation=swipe]/swipe-tray:type-caption-100 touch:group-data-[presentation=swipe]/swipe-tray:text-muted-foreground"

// stateCaptionsClass pairs a stateful button's captions with its faces by
// position (button.IconProps.States renders a face per state, hidden but for
// the current one), so a caption hides with its face wherever the state
// changes it, script or morph, and reads no state names.
const stateCaptionsClass = "group-has-[[data-slot=button-icon-state]:nth-child(1)[hidden]]/swipe-action:[&>:nth-child(1)]:hidden" +
	" group-has-[[data-slot=button-icon-state]:nth-child(2)[hidden]]/swipe-action:[&>:nth-child(2)]:hidden" +
	" group-has-[[data-slot=button-icon-state]:nth-child(3)[hidden]]/swipe-action:[&>:nth-child(3)]:hidden"

// toggleCaptionsClass shows a toggle's first caption while it is not
// pressed and its second while it is, following its aria-pressed.
const toggleCaptionsClass = "group-has-[[aria-pressed=true]]/swipe-action:[&>:nth-child(1)]:hidden not-group-has-[[aria-pressed=true]]/swipe-action:[&>:nth-child(2)]:hidden"

const ChevronClass = "pointer-events-none hidden touch:absolute touch:top-1/2 touch:-translate-y-1/2 touch:block touch:size-4 touch:right-[max(var(--swipe-cue-padding,6px),calc(var(--swipe-distance,0px)-var(--swipe-cue-padding,6px)-16px))] touch:scale-0 touch:opacity-0 touch:[transform:scaleX(var(--swipe-chevron-direction,1))] touch:transition-[scale,opacity,transform] touch:duration-180 touch:ease-[cubic-bezier(0.34,1.9,0.64,1)] touch:group-data-[swipe-dragging]/swipe:scale-[var(--swipe-chevron-opacity,0)] touch:group-data-[swipe-dragging]/swipe:opacity-[var(--swipe-chevron-opacity,0)] touch:motion-reduce:transition-none touch:text-muted-foreground touch:group-data-[swipe-side=right]/swipe-tray:right-auto touch:group-data-[swipe-side=right]/swipe-tray:left-[max(var(--swipe-cue-padding,6px),calc(var(--swipe-distance,0px)-var(--swipe-cue-padding,6px)-16px))] touch:group-data-[swipe-dragging]/swipe:group-data-[swipe=armed]/swipe:scale-0 touch:group-data-[swipe-dragging]/swipe:group-data-[swipe=armed]/swipe:opacity-0"

// At rest, the native hit area fills the padded action cell. The complete
// circle-and-caption group scales around the action cell center during reveal.
const ActionButtonClass = "touch:group-data-[presentation=swipe]/swipe-tray:static touch:group-data-[presentation=swipe]/swipe-tray:before:absolute touch:group-data-[presentation=swipe]/swipe-tray:before:inset-0 touch:group-data-[presentation=swipe]/swipe-tray:before:content-[''] touch:group-data-[presentation=swipe]/swipe-tray:h-14 touch:group-data-[presentation=swipe]/swipe-tray:w-full touch:group-data-[presentation=swipe]/swipe-tray:min-w-14 touch:group-data-[presentation=swipe]/swipe-tray:bg-transparent touch:group-data-[presentation=swipe]/swipe-tray:text-secondary-foreground touch:group-data-[presentation=swipe]/swipe-tray:[&>svg]:relative touch:group-data-[presentation=swipe]/swipe-tray:[&>svg]:z-1 touch:group-data-[presentation=swipe]/swipe-tray:[&>[data-slot=button-icon-state]]:relative touch:group-data-[presentation=swipe]/swipe-tray:[&>[data-slot=button-icon-state]]:z-1 touch:group-data-[presentation=swipe]/swipe-tray:col-start-1 touch:group-data-[presentation=swipe]/swipe-tray:row-start-1" +
	" touch:group-data-[presentation=swipe]/swipe-tray:data-swipe-commit:text-primary-foreground" +
	" touch:group-data-[presentation=swipe]/swipe-tray:data-swipe-commit:data-[optimistic-delete]:text-white" +
	" touch:group-data-[presentation=swipe]/swipe-tray:data-swipe-commit:data-[optimistic-read-state]:text-white"

const SurfaceClass = "hidden pointer-events-none touch:group-data-[presentation=swipe]/swipe-tray:block touch:group-data-[presentation=swipe]/swipe-tray:relative touch:group-data-[presentation=swipe]/swipe-tray:z-0 touch:group-data-[presentation=swipe]/swipe-tray:col-start-1 touch:group-data-[presentation=swipe]/swipe-tray:row-start-1 touch:group-data-[presentation=swipe]/swipe-tray:h-14 touch:group-data-[presentation=swipe]/swipe-tray:w-full touch:group-data-[presentation=swipe]/swipe-tray:min-w-14 touch:group-data-[presentation=swipe]/swipe-tray:squircle-pill touch:group-data-[presentation=swipe]/swipe-tray:bg-secondary" +
	" touch:group-data-[presentation=swipe]/swipe-tray:group-has-[[data-swipe-commit]]/swipe-action:bg-primary touch:group-data-[presentation=swipe]/swipe-tray:group-has-[[data-swipe-commit][data-optimistic-delete]]/swipe-action:bg-destructive touch:group-data-[presentation=swipe]/swipe-tray:group-has-[[data-swipe-commit][data-optimistic-read-state]]/swipe-action:bg-info"
