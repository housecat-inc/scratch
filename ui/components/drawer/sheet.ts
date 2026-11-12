// Dragging a bottom sheet -- a self-controlled modal drawer with
// data-compact-side=bottom (drawer.templ; ./records' SHEET) -- down to
// dismiss it, the way iOS's own sheets go: the sheet follows the finger one
// to one on the way down and gives only grudgingly on the way up. Flicking it down closes it and
// flicking it up springs it back open, wherever it is let go; otherwise
// letting go past a third of its height closes it, and anything less springs
// it back open. A fling's close is quicker the faster the flick and the less
// of the sheet is left to go, and always quicker than a close from a tap or a
// slow drag. A grab during the sheet's own slide takes it from where it is drawn. The
// scrim fades with the drag, and from wherever the drag left it finishes
// over the shared scrim timing (ui/css/theme.css). Under
// prefers-reduced-motion nothing animates: the sheet closes or snaps back at
// once.
//
// The pan is ui/dom/pan.ts's. This is the sheet's part of it: which pans the
// sheet claims, and what a drag does. A pan on the grabber, whose
// touch-action is none, is the sheet's up or down. Anywhere else a downward
// pan is the sheet's too, unless it starts in a scroller that is scrolled
// down: then the content scrolls, as it does on iOS, and a drag at the top of
// the content hands off to the sheet. A mouse drags by the grabber alone, and
// a field or a popover is never dragged (pan.ts claims nothing there).
//
// The browser has to be kept from claiming a downward pan as a scroll before
// it is known to be a drag. Chrome is told up front: while the sheet's
// content is at the top (no data-drawer-scrolled on the aside, which the
// scroll listener below keeps), everything in the sheet allows only pan-x
// pan-down, which leaves a finger moving down to the pointer events. Safari
// reads no pan-down, so pan.ts cancels the sheet's touchmove, bound on each
// sheet, from the first move more down than sideways, well inside the slop,
// before Safari can start its own overscroll.
//
// The drag moves the sheet by its translate alone, from the translate it is
// drawn at when the pan becomes the sheet's, which is 0 at rest and the
// slide's own value mid-slide. It fades the scrim by its background's alpha
// alone -- --scrim mixed with transparent -- and leaves the scrim's opacity
// to the scrim utility, which shows and hides it by opacity. Both are
// inline, with data-drawer-dragging on each holding off their transitions
// until the finger lifts. Springing back drops the inline colour, and the
// scrim utility eases it back. Closing keeps it while the opacity fades out,
// so the scrim never darkens on the way out, and the next open drops it.
// Closing is drawer.ts's close(), so focus, the key layer and the inert page
// come back the same way as for Escape or a tap on the scrim. pan.ts swallows
// the click that ends a drag, so dragging from a link does not follow it.

import { clamp, hide, supportsTopLayer } from "../../dom/dom";
import { cancelTouchOn, type Claim, type PanAt, reducedMotion, register } from "../../dom/pan";
import { flingMs, lift, shut } from "../../dom/panrules";
import { compact } from "../../dom/viewport";
import { close } from "./drawer";
import { backdrop, isOpen, SHEET } from "./records";

const GRABBER = "[data-slot=drawer-grabber]";

// Past this fraction of the sheet's height, letting go closes it.
const CLOSE_AT = 1 / 3;
// A release faster than this, in px/ms, is a fling: down closes the sheet
// and up keeps it open.
const FLING = 0.5;
// The most an upward drag lifts the sheet, however far the finger goes.
const LIFT_PX = 24;
// A fling's slide is the sheet's own slide, --drawer-duration
// (drawer.templ), for the whole sheet at refPxPerMs; shorter in proportion
// for less of the sheet left and for a faster flick; and kept between
// minMs and maxMs. Matching the finger's speed exactly is no use as a rule:
// over most of a sheet's height, at the speeds a thumb flicks (0.5 to 3
// px/ms), that comes out longer than the slide, so a flick would close no
// quicker than a tap.
const FLING_TIMING = { maxMs: 280, minMs: 120, refPxPerMs: 1, slideMs: 350 };

// from is the translate the sheet was drawn at when the pan became its.
type Drag = { back: HTMLElement | null; from: number; height: number; y: number };

let drag: Drag | null = null;

// scrollerAt is the nearest scroller between target and sheet that has
// something to scroll, or null.
function scrollerAt(target: Element, sheet: Element): HTMLElement | null {
  for (let el: Element | null = target; el && el !== sheet; el = el.parentElement) {
    if (!(el instanceof HTMLElement)) continue;
    const overflow = getComputedStyle(el).overflowY;
    if ((overflow === "auto" || overflow === "scroll") && el.scrollHeight > el.clientHeight) return el;
  }
  return null;
}

// drawnY is how far below where it rests open the sheet is drawn right now,
// in px, read off its computed translate, which mid-slide is the
// transition's own value. That value is a percentage of the sheet's height
// (translate-y-full), or a calc() of one and px, so each term is resolved
// against height and summed.
function drawnY(sheet: HTMLElement, height: number): number {
  const y = getComputedStyle(sheet).translate.split(/ (?![^(]*\))/)[1] ?? "";
  let px = 0;
  for (const [, n, unit] of y.replaceAll(/\s-\s/g, " + -").matchAll(/(-?[\d.]+(?:e[-+]?\d+)?)(px|%)/g)) {
    const v = parseFloat(n) || 0;
    px += unit === "%" ? (v * height) / 100 : v;
  }
  return px;
}

// scrimAt is the scrim's colour at fraction f of its strength: --scrim mixed
// with transparent, which scales its alpha.
function scrimAt(f: number): string {
  return `color-mix(in oklab, var(--scrim) ${(f * 100).toFixed(1)}%, transparent)`;
}

function place(sheet: HTMLElement, d: Drag): void {
  sheet.style.translate = `0 ${d.y}px`;
  if (d.back) d.back.style.backgroundColor = scrimAt(clamp(1 - d.y / d.height, 0, 1));
}

// unplace hands the sheet and the scrim back to their transitions. A sheet
// that is closing keeps the scrim's faded colour for its fade out.
function unplace(sheet: HTMLElement, d: Drag, closing: boolean): void {
  sheet.removeAttribute("data-drawer-dragging");
  d.back?.removeAttribute("data-drawer-dragging");
  sheet.style.removeProperty("translate");
  if (!closing) d.back?.style.removeProperty("background-color");
}

function claim(target: Element, event: PointerEvent): Claim | null {
  const sheet = target.closest<HTMLElement>(SHEET);
  if (!sheet || !isOpen(sheet) || !compact().matches) return null;
  const grabber = !!target.closest(GRABBER);
  if (!grabber) {
    if (event.pointerType === "mouse") return null;
    const scroller = scrollerAt(target, sheet);
    if (scroller && scroller.scrollTop > 0) return null;
  }
  return { el: sheet, grabber, kind: "sheet" };
}

// start reads where the sheet is drawn before data-drawer-dragging drops its
// transition, and places it there in the same frame, so a grab mid-slide
// holds it where it is. A popover open from the sheet -- a menu, a tooltip --
// is anchored to something that is about to move, and is in the top layer,
// which does not move with it, so it closes. Only the open ones are looked
// at: every row's tooltip is a popover, and a drag starts on a pointer move.
function start(sheet: HTMLElement): void {
  if (supportsTopLayer()) {
    for (const popover of sheet.querySelectorAll<HTMLElement>(":popover-open")) hide(popover);
  }
  const back = backdrop(sheet);
  const height = sheet.getBoundingClientRect().height || 1;
  const from = drawnY(sheet, height);
  const d = { back, from, height, y: from };
  drag = d;
  sheet.setAttribute("data-drawer-dragging", "");
  back?.setAttribute("data-drawer-dragging", "");
  place(sheet, d);
}

function move(sheet: HTMLElement, at: PanAt): void {
  const d = drag;
  if (!d) return;
  d.y = lift(d.from + at.dy, LIFT_PX);
  place(sheet, d);
}

function end(sheet: HTMLElement, at: PanAt): void {
  const d = drag;
  drag = null;
  if (!d) return;
  const release = { height: d.height, v: at.v, y: d.y };
  const closing = shut(release, CLOSE_AT, FLING);
  if (closing && at.v > FLING && !reducedMotion()) {
    sheet.style.setProperty("--drawer-duration", Math.round(flingMs(release, FLING_TIMING)) + "ms");
  }
  unplace(sheet, d, closing);
  if (closing) close(sheet, { restoreFocus: true });
}

// A pan the browser took after all springs the sheet back.
function cancel(sheet: HTMLElement): void {
  const d = drag;
  drag = null;
  if (d) unplace(sheet, d, false);
}

register({ cancel, claim, end, move, start });

// revealCurrent scrolls the row for the page the sheet's list is open on
// (aria-current=page, else any aria-current) into sight as the sheet opens,
// when it is out of sight: to the lower third of its scroller, where a thumb
// reaches it, at once, since the sheet's own slide is the motion. A row
// inside a closed folder or section is not shown and is left alone. It is
// checkVisibility that says so: the browser hides a closed <details>'
// content by content-visibility, under which a row still has boxes. Safari
// before 17.4 has no checkVisibility, and there a box is the best sign.
function revealCurrent(sheet: HTMLElement): void {
  const row = sheet.querySelector('[aria-current="page"]') ?? sheet.querySelector("[aria-current]");
  const scroller = row ? scrollerAt(row, sheet) : null;
  if (!row || !scroller) return;
  if (!("checkVisibility" in Element.prototype ? row.checkVisibility() : row.getClientRects().length > 0)) return;
  const at = row.getBoundingClientRect();
  const box = scroller.getBoundingClientRect();
  if (at.top >= box.top && at.bottom <= box.bottom) return;
  scroller.scrollTop += at.top + at.height / 2 - (box.top + (box.height * 2) / 3);
}

// A fling's shorter slide and a drag's faded scrim are for that close alone:
// the next open slides over the sheet's own duration and fades the scrim in
// at full strength. drawer:toggle fires before the browser next computes
// styles, so the open starts with both.
document.addEventListener("drawer:toggle", (event) => {
  const sheet = event.target;
  if (sheet instanceof HTMLElement && sheet.matches(SHEET) && event.detail.open) {
    sheet.style.removeProperty("--drawer-duration");
    backdrop(sheet)?.style.removeProperty("background-color");
    if (compact().matches) revealCurrent(sheet);
  }
});

// A tap on the grabber closes the sheet, as a drag past a third of it does.
// The grabber is a 44px strip, a target a thumb finds without looking; with
// one height to open to, there is no other height a tap could move it to. A
// drag that started on it ends in a click pan.ts swallows. A short drag that
// stays inside the slop is no drag to pan.ts, and its click goes through,
// as a wobbling tap's does on a row. But the grabber is there to be dragged:
// a press that moved TAP_PX or more was a drag that never got going, and
// leaves the sheet where it is. Only a press that held still is a tap.
const TAP_PX = 4;

let press: { id: number; moved: boolean; x: number; y: number } | null = null;

document.addEventListener("pointerdown", (event) => {
  const onGrabber = event.target instanceof Element && !!event.target.closest(GRABBER);
  press = onGrabber ? { id: event.pointerId, moved: false, x: event.clientX, y: event.clientY } : null;
}, { capture: true, passive: true });

document.addEventListener("pointermove", (event) => {
  if (!press || press.moved || event.pointerId !== press.id) return;
  press.moved = Math.hypot(event.clientX - press.x, event.clientY - press.y) >= TAP_PX;
}, { capture: true, passive: true });

document.addEventListener("click", (event) => {
  const grabber = event.target instanceof Element ? event.target.closest(GRABBER) : null;
  const moved = press?.moved ?? false;
  press = null;
  const sheet = grabber?.closest<HTMLElement>(SHEET);
  if (sheet && !moved && isOpen(sheet) && compact().matches) close(sheet, { restoreFocus: true });
});

// Whether any scroller in the sheet is scrolled down, for the touch-action
// that leaves a downward pan at the top to the sheet. It is any, not the one
// that scrolled last: a second scroller back at its top -- the account menu,
// a row that scrolls sideways -- leaves the sheet's list where it was.
const scrolledIn = new WeakMap<Element, Set<Element>>();

function onScroll(event: Event): void {
  const sheet = event.currentTarget;
  const target = event.target;
  if (!(sheet instanceof Element) || !(target instanceof Element)) return;
  let scrolled = scrolledIn.get(sheet);
  if (!scrolled) scrolledIn.set(sheet, scrolled = new Set());
  if (target.scrollTop > 0) scrolled.add(target);
  else scrolled.delete(target);
  for (const el of scrolled) if (!sheet.contains(el)) scrolled.delete(el);
  sheet.toggleAttribute("data-drawer-scrolled", scrolled.size > 0);
}

// Bound on each sheet rather than on the document, so a touch anywhere else
// on the page is not held up waiting on a script that might cancel it, and
// a scroll anywhere else -- the mail list's, every frame -- runs nothing
// here. Scroll does not bubble, but it passes the sheet on its way down.
const bound = new WeakSet<Element>();

function bind(): void {
  for (const sheet of document.querySelectorAll<HTMLElement>(SHEET)) {
    if (bound.has(sheet)) continue;
    bound.add(sheet);
    cancelTouchOn(sheet);
    sheet.addEventListener("scroll", onScroll, { capture: true, passive: true });
  }
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", bind);
else bind();
document.addEventListener("htmx:load", bind);
