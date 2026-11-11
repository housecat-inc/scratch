// Sequences a tour rendered by tour.templ: the welcome dialog, then each
// step's popover pinned to its anchor. Progress is posted to the root's
// data-save-url, with whatever state the page has handed the tour in a
// tour:state event -- the chat a step is about -- so a reload can put the page
// back as it was. Before each step a tour:reveal event asks the page to show
// what the step points at, since only the page knows how to open its own
// drawers and rails; the step then waits for its anchor to be visible. A step
// whose data-ready names an element opens at once on its waiting text and
// changes to its own when that element shows inside the anchor. Starting the
// tour also sends the welcome dialog's form, if it has one and the user filled
// it in.
//
// An open step is a modal layer of the key router (ui/dom/keys.ts) over its
// anchor and its popover. An overlay dims the rest of the page and swallows
// clicks anywhere but the anchor; the router keeps focus and keys inside the
// step, and the page's shortcuts never act behind the overlay, between steps
// included.

import { constVoid } from "effect/Function";
import { isTabbable, tabbable } from "tabbable";
import { CONTROL, supportsTopLayer } from "../../dom/dom";
import { reportUnlessAborted } from "../../dom/failures";
import { layer } from "../../dom/keys";
import { docked as dockedQuery } from "../../dom/viewport";
import { place, track, untrack } from "../../floating/floating";

const ROOT = "[data-slot=tour]";
const WELCOME = "[data-slot=tour-welcome]";
const WELCOME_FORM = "[data-slot=tour-welcome-form]";
const STEP = "[data-slot=tour-step]";
const ARROW = "[data-slot=tour-arrow]";
const TITLE = "[data-slot=tour-title]";
const DESCRIPTION = "[data-slot=tour-description]";
const ANNOUNCER = "[data-slot=tour-announcer]";
const OVERLAY = "[data-slot=tour-overlay]";
const DIM = "[data-slot=tour-overlay-dim]";
const READY_TEXT = "template[data-slot=tour-ready]";
const PRIMARY = "[data-action=next], [data-action=done]";
const WELCOME_STEP = "welcome";

const ANCHOR_TIMEOUT = 5000;
const SETTLE_FRAMES = 60;
const SETTLE_TIMEOUT = 2000;
// How many of Tailwind's --spacing increments (ui/css/theme.css, 0.25rem by
// default) clear the hole's edge before a step's arrow tip: the design asks
// for one, in the dark area outside the hole.
const TIP_GAP_SPACING = 1;
// How many --spacing increments a padded step's hole grows by on every side.
const PAD_SPACING = 1;

export type RevealDetail = { reveal: string };

// StateDetail is what a page hands the tour to save with each step, such as
// the chat a step is about; an empty value leaves the saved one alone.
export type StateDetail = Record<string, string>;

declare global {
  interface GlobalEventHandlersEventMap {
    // A page's state for the tour, on the document; a script may send none.
    "tour:state": CustomEvent<StateDetail | null>;
  }
}

type Outcome = "completed" | "skipped";

// Tour's anchor is the element the overlay's hole is over. It outlives its
// step: between steps the hole stays on it until the next anchor settles, and
// the end of the tour hands focus back to it. dim is the overlay's paint, and
// takes its pointer events; resized redraws the hole when the anchor changes
// size. pad is how far the hole grows past the anchor on every side, in px,
// for the step that put it there, so it holds between steps too. ready
// watches an open waiting step's anchor for its data-ready. state is what the
// page has handed the tour to save. aborted's signal goes to every listener
// the tour adds, and finish aborts it, so a finished tour leaves none behind
// on the window, the document or the docked query.
type Tour = {
  aborted: AbortController;
  anchor: HTMLElement | null;
  current: HTMLElement | null;
  dim: SVGPathElement | null;
  finished: boolean;
  frame: number;
  generation: number;
  index: number;
  observer: MutationObserver;
  overlay: SVGSVGElement | null;
  pad: number;
  ready: MutationObserver;
  resized: ResizeObserver;
  root: HTMLElement;
  state: StateDetail;
  steps: HTMLElement[];
  welcome: HTMLDialogElement | null;
};

function anchorSelector(name: string): string {
  return '[data-tour-anchor="' + CSS.escape(name) + '"]';
}

// shown is whether el renders with a box to point at: checkVisibility passes
// an element with no width or no height, such as an empty block, which would
// leave a step pinned to a line and a hole around nothing.
function shown(el: HTMLElement): boolean {
  if (!el.checkVisibility()) return false;
  const r = el.getBoundingClientRect();
  return r.width > 0 && r.height > 0;
}

function visible(selector: string): HTMLElement | null {
  return Array.from(document.querySelectorAll<HTMLElement>(selector)).find(shown) ?? null;
}

function visibleAnchor(name: string): HTMLElement | null {
  return visible(anchorSelector(name));
}

// waitFor is the first visible element selector matches, once there is one,
// or null after timeout ms. It looks again every frame, which catches both a
// mutation and an element gaining its size from layout alone, with no
// mutation to say so.
function waitFor(selector: string, timeout: number): Promise<HTMLElement | null> {
  return new Promise((resolve) => {
    const found = visible(selector);
    if (found) {
      resolve(found);
      return;
    }
    const tick = (): void => {
      const el = visible(selector);
      if (!el) {
        frame = requestAnimationFrame(tick);
        return;
      }
      window.clearTimeout(timer);
      resolve(el);
    };
    const timer = window.setTimeout(() => {
      cancelAnimationFrame(frame);
      resolve(null);
    }, timeout);
    let frame = requestAnimationFrame(tick);
  });
}

// movesWith is whether an animation runs on el or an ancestor and will end.
// A transition looks still in its first frame, so watching the rect alone
// places a popup before the rail it points into has started to slide.
function movesWith(animation: Animation, el: Element): boolean {
  const effect = animation.effect;
  if (!(effect instanceof KeyframeEffect) || effect.target?.contains(el) !== true) return false;
  return effect.getComputedTiming().endTime !== Infinity;
}

// A step waits for its anchor to stop moving: the chat rail slides in, and a
// hole cut mid-slide stays where the anchor was, since drawHole runs only on
// resize and scroll. Reading the rect first flushes style, which
// starts any transition a reveal has just asked for, so getAnimations sees it.
// An animation that never ends, such as a paused one, is waited on for
// SETTLE_TIMEOUT at most, so the step still opens.
async function settle(el: HTMLElement): Promise<void> {
  el.getBoundingClientRect();
  const animations = document.getAnimations().filter((a) => movesWith(a, el)).map((a) => a.finished);
  await Promise.race([
    Promise.allSettled(animations),
    new Promise<void>((resolve) => window.setTimeout(resolve, SETTLE_TIMEOUT)),
  ]);
  let last = "";
  for (let frame = 0; frame < SETTLE_FRAMES; frame++) {
    const r = el.getBoundingClientRect();
    const now = [r.left, r.top, r.width, r.height].join();
    if (now === last) return;
    last = now;
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
  }
}

// settledAnchor is the step's anchor once it has stopped moving. The swap that
// fills the chat rail can land while the rail slides in, replacing the anchor
// the step was waiting on, so a detached anchor is looked up again rather than
// pinned to.
async function settledAnchor(name: string): Promise<HTMLElement | null> {
  let anchor = await waitFor(anchorSelector(name), ANCHOR_TIMEOUT);
  while (anchor) {
    await settle(anchor);
    if (anchor.isConnected) return anchor;
    anchor = await waitFor(anchorSelector(name), ANCHOR_TIMEOUT);
  }
  return null;
}

// keepalive lets a post outlive a navigation that follows it. A failed post
// is dropped: a lost save resumes the next load one step earlier, which is
// harmless. A post the network cut off is expected; any other failure is
// reported. The body's data-tour-saving counts the posts in flight, so a test
// reads what the tour sent only once every post has landed; it is on the body
// because a tour that ends removes its root while its last post is still out.
function post(url: string, fields: Record<string, string>, operation: string): void {
  const body = document.body;
  body.dataset.tourSaving = String(Number(body.dataset.tourSaving ?? 0) + 1);
  fetch(url, { body: new URLSearchParams(fields), keepalive: true, method: "POST" })
    .catch(reportUnlessAborted({ operation }))
    .finally(() => {
      const left = Number(body.dataset.tourSaving) - 1;
      if (left > 0) body.dataset.tourSaving = String(left);
      else delete body.dataset.tourSaving;
    });
}

function save(tour: Tour, fields: Record<string, string>): void {
  const url = tour.root.dataset.saveUrl;
  if (url) post(url, fields, "tour.save");
}

// sendWelcomeForm sends the welcome dialog's form, when it has one and any of
// its fields is filled in. Only starting the tour calls it: what the user typed
// goes when they chose to go on, and every way of skipping leaves it unsent.
function sendWelcomeForm(tour: Tour): void {
  const form = tour.welcome?.querySelector<HTMLFormElement>(WELCOME_FORM);
  if (!form) return;
  const fields: Record<string, string> = {};
  for (const [name, value] of new FormData(form)) {
    if (typeof value === "string") fields[name] = value;
  }
  if (Object.values(fields).some((value) => value.trim() !== "")) post(form.action, fields, "tour.welcome-form");
}

// saveStep saves the step the tour is on, with the page's state beside it.
function saveStep(tour: Tour): void {
  const name = tour.steps[tour.index]?.dataset.name;
  if (name) save(tour, { ...tour.state, step: name });
}

// A step puts focus on the page rather than on its popup, whose dialog labels
// would otherwise read it out, so the announcer reads each step: its title,
// then its body. It holds only the open step's text: closeStep empties it, so
// a step that opens again -- on widening, after a narrowing closed it --
// changes the region and is read again, where writing the same text twice
// would not be.
function announce(tour: Tour, step: HTMLElement): void {
  const announcer = tour.root.querySelector<HTMLElement>(ANNOUNCER);
  if (!announcer) return;
  const title = step.querySelector(TITLE)?.textContent ?? "";
  const body = step.querySelector(DESCRIPTION)?.textContent ?? "";
  announcer.textContent = title + ". " + body;
}

// tabbables is what Tab stops on inside el, el itself first when it is one,
// leaving out whatever is not rendered.
function tabbables(el: HTMLElement): HTMLElement[] {
  return tabbable(el, { displayCheck: "full", includeContainer: true }).filter(
    (each): each is HTMLElement => each instanceof HTMLElement,
  );
}

// namedFocus is the first focusable in the element a step's data-focus names,
// the anchor itself or one inside it, or null when the step names none or it
// is not showing. The prompts step names its own anchor, so this is the first
// prompt.
function namedFocus(step: HTMLElement, anchor: HTMLElement): HTMLElement | null {
  const name = step.dataset.focus ?? "";
  if (!name) return null;
  const selector = anchorSelector(name);
  const candidates = [
    ...(anchor.matches(selector) ? [anchor] : []),
    ...Array.from(anchor.querySelectorAll<HTMLElement>(selector)),
  ];
  const named = candidates.find((el) => el.checkVisibility());
  return (named && tabbables(named)[0]) ?? null;
}

// focusTarget is where a step puts focus when it opens: its named focus, else
// its own Next or Done button -- the default, so a step that says nothing
// about focus leaves it on the control that moves the tour on, not on a page
// element the user did not ask to interact with.
function focusTarget(step: HTMLElement, anchor: HTMLElement): HTMLElement | null {
  return namedFocus(step, anchor) ?? step.querySelector<HTMLElement>(PRIMARY);
}

function focusStep(tour: Tour): void {
  const step = tour.current;
  if (!step || !tour.anchor) return;
  focusTarget(step, tour.anchor)?.focus({ preventScroll: true });
  if (step.dataset.focus && !namedFocus(step, tour.anchor)) awaitFocus(tour, step);
}

// awaitFocus puts focus on a step's named focus once it shows, for a step
// that opened before it did: the starter prompts fade in with the chat rail,
// each on a visibility transition of its own, which settle does not wait for
// since it watches only the anchor and what holds it, and a prompt still
// hidden takes no focus, so the step falls back to its own button. It looks
// every frame, as waitFor does, until the step moves on or ANCHOR_TIMEOUT
// passes, and moves focus only from that button or the body. Once focus has
// gone anywhere else it stops: focus on that button then is where the user
// went back to, not where the step left it.
function awaitFocus(tour: Tour, step: HTMLElement): void {
  const generation = tour.generation;
  const deadline = performance.now() + ANCHOR_TIMEOUT;
  const own = step.querySelector<HTMLElement>(PRIMARY);
  let moved = false;
  const onFocus = (event: FocusEvent): void => {
    if (event.target !== own) moved = true;
  };
  document.addEventListener("focusin", onFocus, { signal: tour.aborted.signal });
  const look = (): void => {
    if (moved || generation !== tour.generation || performance.now() > deadline) {
      document.removeEventListener("focusin", onFocus);
      return;
    }
    const anchor = anchorNow(tour);
    const named = anchor ? namedFocus(step, anchor) : null;
    if (!named) {
      requestAnimationFrame(look);
      return;
    }
    document.removeEventListener("focusin", onFocus);
    const active = document.activeElement;
    if (!active || active === document.body || active === own) named.focus({ preventScroll: true });
  };
  requestAnimationFrame(look);
}

// release hands focus back to the page as the tour ends. Focus the page has
// already taken, in the last anchor or wherever a click on it sent focus, as
// Give Feedback does to the form it opens, stays put. Otherwise it goes to
// that anchor when it takes focus, else to the body rather than staying on a
// popover about to go.
function release(tour: Tour): void {
  const last = tour.anchor;
  const active = document.activeElement;
  if (active && active !== document.body && !tour.root.contains(active)) return;
  const target = last?.matches("[data-slot=button-surface], [data-slot=nav-item-surface]") === true
    ? tabbables(last)[0]
    : last;
  if (target?.isConnected === true && isTabbable(target)) target.focus({ preventScroll: true });
  else if (active instanceof HTMLElement) active.blur();
}

// A corner's radii, across and down, in px.
type Radius = { x: number; y: number };

// Corners runs from the top left clockwise.
type Corners = [Radius, Radius, Radius, Radius];

// cornerLength is one of a computed border radius's lengths in px: a
// percentage is of the side it runs along, and anything else unreadable, such
// as a calc() mixing the two, counts as square.
function cornerLength(value: string, side: number): number {
  const n = parseFloat(value) || 0;
  return value.endsWith("%") ? (n * side) / 100 : n;
}

// fit scales every corner down by one factor until adjacent corners fit
// along each side of a width by height box, as the browser does with radii
// too large for their box (css-backgrounds, "Overlapping Curves").
function fit([tl, tr, br, bl]: Corners, width: number, height: number): Corners {
  const room = (side: number, sum: number): number => (sum > side ? side / sum : 1);
  const scale = Math.min(
    room(width, tl.x + tr.x),
    room(height, tr.y + br.y),
    room(width, br.x + bl.x),
    room(height, bl.y + tl.y),
  );
  const scaled = (r: Radius): Radius => ({ x: r.x * scale, y: r.y * scale });
  return [scaled(tl), scaled(tr), scaled(br), scaled(bl)];
}

// cornerRadii is the radii the browser draws el's corners with, fitted to its
// width by height box, since a computed radius can be larger than the box, as
// a pill's is.
function cornerRadii(el: HTMLElement, width: number, height: number): Corners {
  const style = getComputedStyle(el);
  const radius = (value: string): Radius => {
    const [x = "0", y = x] = value.split(" ");
    return { x: cornerLength(x, width), y: cornerLength(y, height) };
  };
  return fit(
    [
      radius(style.borderTopLeftRadius),
      radius(style.borderTopRightRadius),
      radius(style.borderBottomRightRadius),
      radius(style.borderBottomLeftRadius),
    ],
    width,
    height,
  );
}

// A box's edges in viewport px.
type Box = { bottom: number; left: number; right: number; top: number };

// holdsFixed is whether an element with style is the containing block of its
// fixed-position descendants, which otherwise belong to the viewport.
function holdsFixed(style: CSSStyleDeclaration): boolean {
  return style.transform !== "none" || style.perspective !== "none" || style.filter !== "none"
    || style.backdropFilter !== "none" || /layout|paint|strict|content/.test(style.contain)
    || /transform|perspective|filter/.test(style.willChange);
}

// shownBox is the part of el's box r that shows through the ancestors that
// clip it, as a scrolled list clips its rows: each one whose overflow is not
// visible, or that contains its paint, cuts at its padding edge. Only an
// ancestor in el's chain of containing blocks clips it, so a fixed element,
// such as the chat rail, escapes every ancestor up to one that holds fixed
// descendants, and an absolute one every ancestor up to a positioned one. An
// inline box never clips, and the body's and root's overflow belong to the
// viewport, which the dim covers anyway.
function shownBox(el: HTMLElement, r: DOMRect): Box {
  const box = { bottom: r.bottom, left: r.left, right: r.right, top: r.top };
  let position = getComputedStyle(el).position;
  for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) {
    const style = getComputedStyle(p);
    const holds = holdsFixed(style);
    if (position === "fixed" && !holds) continue;
    if (position === "absolute" && style.position === "static" && !holds) continue;
    position = style.position;
    if (style.display === "inline" || style.display === "contents") continue;
    const paint = /paint|strict|content/.test(style.contain);
    const q = p.getBoundingClientRect();
    const left = q.left + p.clientLeft;
    const top = q.top + p.clientTop;
    if (paint || style.overflowX !== "visible") {
      box.left = Math.max(box.left, left);
      box.right = Math.min(box.right, left + p.clientWidth);
    }
    if (paint || style.overflowY !== "visible") {
      box.top = Math.max(box.top, top);
      box.bottom = Math.min(box.bottom, top + p.clientHeight);
    }
  }
  return box;
}

// grow adds by to both radii of every corner. An outline grown by that much
// on every side then stays that far from the one inside it all the way round,
// a square corner included.
function grow([tl, tr, br, bl]: Corners, by: number): Corners {
  const grown = (r: Radius): Radius => ({ x: r.x + by, y: r.y + by });
  return [grown(tl), grown(tr), grown(br), grown(bl)];
}

// squareCut squares every corner on an edge of r that box has cut, where the
// anchor runs on out of sight.
function squareCut([tl, tr, br, bl]: Corners, r: DOMRect, box: Box): Corners {
  const top = box.top > r.top;
  const right = box.right < r.right;
  const bottom = box.bottom < r.bottom;
  const left = box.left > r.left;
  const square = { x: 0, y: 0 };
  return [
    top || left ? square : tl,
    top || right ? square : tr,
    bottom || right ? square : br,
    bottom || left ? square : bl,
  ];
}

// roundedRect is a path around a rect from its top edge clockwise, its arcs
// running top right, bottom right, bottom left, top left.
function roundedRect(x: number, y: number, width: number, height: number, [tl, tr, br, bl]: Corners): string {
  const arc = (r: Radius, toX: number, toY: number): string => "A" + r.x + " " + r.y + " 0 0 1 " + toX + " " + toY;
  return "M" + (x + tl.x) + " " + y + "H" + (x + width - tr.x) + arc(tr, x + width, y + tr.y)
    + "V" + (y + height - br.y) + arc(br, x + width - br.x, y + height)
    + "H" + (x + bl.x) + arc(bl, x, y + height - bl.y)
    + "V" + (y + tl.y) + arc(tl, x + tl.x, y) + "Z";
}

// drawHole cuts the overlay's hole over the anchor where it is now; an anchor
// no longer rendered, such as prompts hidden by the message they sent, leaves
// no hole. It runs when the anchor changes size, the page scrolls or the
// window resizes, and when a step opens on an anchor that has stopped moving;
// the open step's popup moves with the hole.
//
// The dim's hole is the anchor's own outline, corners included, and no wider
// unless the step pads it, by tour.pad on every side. Some anchors sit flush
// against their neighbours -- the row below Important, the mail pane beside
// the chat rail -- so any margin around them would light part of a neighbour
// up as though the step pointed at it too. The hole, padded or not, is cut to
// the part that shows, squared off where an ancestor cuts it: a row scrolled
// up under the sidebar's header leaves the header dimmed, and one scrolled
// out of sight leaves no hole. The dim takes the overlay's clicks, so one
// anywhere outside the hole, a dimmed rounded corner included, is swallowed.
function drawHole(tour: Tour): void {
  if (!tour.dim) return;
  const viewport = "M0 0H" + window.innerWidth + "V" + window.innerHeight + "H0Z";
  let d = viewport;
  const anchor = tour.anchor?.isConnected === true && tour.anchor.checkVisibility() ? tour.anchor : null;
  if (anchor) {
    const r = anchor.getBoundingClientRect();
    const pad = tour.pad;
    const padded = new DOMRect(r.x - pad, r.y - pad, r.width + 2 * pad, r.height + 2 * pad);
    const box = shownBox(anchor, padded);
    const width = box.right - box.left;
    const height = box.bottom - box.top;
    if (width > 0 && height > 0) {
      const corners = fit(squareCut(grow(cornerRadii(anchor, r.width, r.height), pad), padded, box), width, height);
      d += roundedRect(box.left, box.top, width, height, corners);
    }
  }
  if (tour.dim.getAttribute("d") === d) return;
  tour.dim.setAttribute("d", d);
  // place never rejects: a placement that fails is reported where it ran.
  if (anchor && tour.current) void place(anchor, tour.current);
}

// overlayShown is whether the dim is up, which it stays between steps.
function overlayShown(tour: Tour): boolean {
  return tour.overlay?.hasAttribute("data-open") === true;
}

// hold points the hole at anchor and redraws it whenever anchor changes size.
function hold(tour: Tour, anchor: HTMLElement): void {
  tour.resized.disconnect();
  tour.anchor = anchor;
  tour.resized.observe(anchor);
  drawHole(tour);
}

function showOverlay(tour: Tour): void {
  if (!tour.overlay) return;
  drawHole(tour);
  tour.overlay.toggleAttribute("data-open", true);
  cancelAnimationFrame(tour.frame);
  const track = () => {
    if (tour.aborted.signal.aborted || !tour.root.isConnected || !overlayShown(tour)) return;
    drawHole(tour);
    tour.frame = requestAnimationFrame(track);
  };
  tour.frame = requestAnimationFrame(track);
}

function hideOverlay(tour: Tour): void {
  cancelAnimationFrame(tour.frame);
  tour.resized.disconnect();
  tour.overlay?.removeAttribute("data-open");
}

// awaitReady shows a waiting step's own title and body once its data-ready
// element shows inside the anchor, which may be at once, as for an answer
// that arrived before a reload. Every change inside the anchor is a chance
// for it to have arrived, since a streamed answer builds up node by node.
function awaitReady(tour: Tour, step: HTMLElement): void {
  const ready = step.dataset.ready ?? "";
  const text = step.querySelector<HTMLTemplateElement>(READY_TEXT);
  if (!ready || !text) return;
  const check = (): void => {
    if (tour.current !== step || !tour.anchor) return;
    const shownIn = Array.from(tour.anchor.querySelectorAll<HTMLElement>(anchorSelector(ready))).some(shown);
    if (!shownIn) return;
    tour.ready.disconnect();
    const arrived = text.content.cloneNode(true);
    if (!(arrived instanceof DocumentFragment)) return;
    for (const slot of [TITLE, DESCRIPTION]) {
      const next = arrived.querySelector(slot);
      if (next) step.querySelector(slot)?.replaceWith(next);
    }
    text.remove();
    announce(tour, step);
  };
  tour.ready = new MutationObserver(check);
  tour.ready.observe(tour.anchor ?? document.body, { characterData: true, childList: true, subtree: true });
  check();
}

// closeStep leaves the overlay up, so moving between steps does not flash the
// page; the hole stays on the last anchor until the next one settles.
function closeStep(tour: Tour): void {
  const step = tour.current;
  tour.current = null;
  tour.observer.disconnect();
  tour.ready.disconnect();
  if (!step) return;
  tour.root.querySelector(ANNOUNCER)?.replaceChildren();
  untrack(step);
  if (step.matches(":popover-open")) step.hidePopover();
}

// spacingPx measures Tailwind's --spacing custom property in pixels. It is a
// CSS length, not necessarily px (0.25rem by default), so a probe element is
// measured rather than its string parsed.
function spacingPx(): number {
  const probe = document.createElement("div");
  probe.style.cssText = "position:fixed;visibility:hidden;height:0;width:var(--spacing);";
  document.body.appendChild(probe);
  const px = probe.getBoundingClientRect().width;
  probe.remove();
  return px;
}

// arrowProtrusion is how far a step's arrow tip reaches past its popup's
// edge. popover.ArrowClass rotates a square 45 degrees with its centre pinned
// to that edge, so half the rotated box's bounding size is the distance from
// edge to tip; measured on the step's own arrow, rather than assumed from its
// class, so a change to ArrowClass's size keeps the tip on target.
function arrowProtrusion(step: HTMLElement): number {
  const arrow = step.querySelector<HTMLElement>(ARROW);
  if (!arrow) return 0;
  const box = arrow.getBoundingClientRect();
  return Math.max(box.width, box.height) / 2;
}

// tipOffset is the side offset (data-side-offset, read by floating.ts's
// place) that lands a step's arrow tip TIP_GAP_SPACING outside the hole's
// edge, pad past the anchor's, in the dark area, on every side. The popup's
// near edge sits this far from the anchor; the arrow's tip reaches back
// toward the anchor by arrowProtrusion from there. A tour-only exception:
// popover's own default side offset is untouched.
function tipOffset(step: HTMLElement, pad: number): number {
  return pad + TIP_GAP_SPACING * spacingPx() + arrowProtrusion(step);
}

async function show(tour: Tour, index: number): Promise<void> {
  if (tour.finished) return;
  closeStep(tour);
  const generation = ++tour.generation;
  const step = tour.steps[index] as HTMLElement | undefined;
  if (!step) {
    finish(tour, "completed");
    return;
  }
  tour.index = index;
  const name = step.dataset.name ?? "";
  tour.root.dataset.step = name;
  document.dispatchEvent(
    new CustomEvent<RevealDetail>("tour:reveal", { detail: { reveal: step.dataset.reveal ?? "none" } }),
  );
  const anchor = await settledAnchor(step.dataset.anchor ?? "");
  if (generation !== tour.generation) return;
  if (!anchor) {
    console.warn("tour: no visible anchor for step", name, step.dataset.anchor);
    await show(tour, index + 1);
    return;
  }
  saveStep(tour);
  step.showPopover();
  tour.pad = step.hasAttribute("data-padded") ? PAD_SPACING * spacingPx() : 0;
  step.dataset.sideOffset = String(Math.round(tipOffset(step, tour.pad)));
  // track never rejects: a placement that fails is reported where it ran.
  void track(anchor, step);
  tour.current = step;
  hold(tour, anchor);
  tour.observer.observe(document.body, { childList: true, subtree: true });
  showOverlay(tour);
  focusStep(tour);
  announce(tour, step);
  awaitReady(tour, step);
}

function next(tour: Tour): void {
  if (tour.index >= tour.steps.length - 1) finish(tour, "completed");
  else void show(tour, tour.index + 1);
}

// pressNext is Next or Done. A step with a named focus -- the prompts step's
// first prompt -- has Next do what that element does, as though the user had
// clicked it: a step that moves on when its anchor is clicked then moves on
// from that click, and any other moves on as well.
function pressNext(tour: Tour): void {
  const step = tour.current;
  const target = step && tour.anchor ? namedFocus(step, tour.anchor) : null;
  if (!step || !target) {
    next(tour);
    return;
  }
  target.click();
  if (step.dataset.advance !== "anchor") next(tour);
}

function finish(tour: Tour, outcome: Outcome): void {
  if (tour.finished) return;
  tour.finished = true;
  tour.aborted.abort();
  tour.generation++;
  tour.observer.disconnect();
  tour.ready.disconnect();
  save(tour, { outcome });
  closeStep(tour);
  hideOverlay(tour);
  if (tour.welcome?.open === true) tour.welcome.close();
  release(tour);
  tour.root.remove();
}

function pause(tour: Tour): void {
  tour.generation++;
  closeStep(tour);
  hideOverlay(tour);
  if (tour.welcome?.open === true) tour.welcome.close();
}

function resume(tour: Tour): void {
  if (tour.finished) return;
  if (tour.index >= 0) {
    void show(tour, tour.index);
    return;
  }
  if (!tour.welcome || tour.welcome.open) return;
  tour.welcome.showModal();
  // A welcome form's first field takes focus, so the user can start typing;
  // without a form, Show me around does.
  const first = tour.welcome.querySelector<HTMLElement>(`${WELCOME_FORM} :is(input, textarea, select)`);
  (first ?? tour.welcome.querySelector<HTMLElement>("[data-action=start]"))?.focus();
}

// An htmx swap can replace the element a step is pinned to -- navigating
// re-renders the sidebar -- so a detached anchor is swapped for its
// replacement rather than left pinning the popup to nothing. A swap that
// takes the focused element with it leaves focus to the router, which puts it
// back on the step's focus target.
function reanchor(tour: Tour): void {
  const step = tour.current;
  if (!step || !tour.anchor) return;
  if (!tour.anchor.isConnected) {
    const replacement = visibleAnchor(step.dataset.anchor ?? "");
    if (!replacement) return;
    hold(tour, replacement);
    // track never rejects: a placement that fails is reported where it ran.
    void track(replacement, step);
    if (step.querySelector(READY_TEXT)) awaitReady(tour, step);
  }
}

function wire(tour: Tour): void {
  const signal = tour.aborted.signal;
  tour.root.addEventListener("click", (event) => {
    if (!(event.target instanceof Element)) return;
    const action = event.target.closest<HTMLElement>("[data-action]")?.dataset.action;
    if (action === undefined) return;
    switch (action) {
      case "start":
        sendWelcomeForm(tour);
        tour.welcome?.close();
        void show(tour, 0);
        break;
      case "skip":
        finish(tour, "skipped");
        break;
      case "next":
      case "done":
        pressNext(tour);
        break;
    }
  }, { signal });
  // Escape on the welcome dialog is the router's, through the tour's layer,
  // which resolves it in keydown and prevents its default, so cancel normally
  // never comes: Chromium can skip it without user activation, and a
  // synthetic Escape never fires it. It stays for a close request that is not
  // a key, such as Android's back, and finish ignores a tour already ended.
  tour.welcome?.addEventListener("cancel", (event) => {
    event.preventDefault();
    finish(tour, "skipped");
  }, { signal });
  // The form's only controls today are textareas, so nothing submits it
  // natively. Guard anyway: a future field (an <input>, say) would submit on
  // Enter, and an empty FormAction resolves to the current page, so a native
  // submit would navigate there. Sending stays solely sendWelcomeForm's job,
  // from the start click above.
  tour.welcome?.querySelector<HTMLFormElement>(WELCOME_FORM)
    ?.addEventListener("submit", (event) => event.preventDefault(), { signal });
  // The hole follows the anchor through scrolling, in any scroller, and a
  // window resize; hold's ResizeObserver covers the anchor's own size.
  window.addEventListener("scroll", () => drawHole(tour), { capture: true, passive: true, signal });
  window.addEventListener("resize", () => drawHole(tour), { signal });
  // A page hands the tour state to save with its steps, and a step already
  // open is saved again with it, so the chat a step is about is kept as soon
  // as the page knows it.
  document.addEventListener("tour:state", (event) => {
    const detail = event.detail ?? {};
    for (const [key, value] of Object.entries(detail)) if (value) tour.state[key] = value;
    if (tour.current) saveStep(tour);
  }, { signal });
  // A press on the dim -- anywhere outside the anchor -- would drop focus to
  // the body, since nothing there takes it, and a click there does nothing,
  // not even for the page's own listeners.
  tour.dim?.addEventListener("mousedown", (event) => event.preventDefault(), { signal });
  tour.dim?.addEventListener("click", (event) => {
    event.preventDefault();
    event.stopPropagation();
  }, { signal });
  // The page's own click handlers run first -- New Chat opens the rail, a
  // prompt fills the composer -- and the tour moves on after them. It never
  // cancels the click.
  // A step that has moved on by the time the timer runs -- Next, Escape, a
  // narrowing -- is not moved on again. Only a click on a control inside the
  // anchor counts: the prompts anchor also holds its heading, and a click
  // there sends nothing. The control is read as the click begins, in the
  // capture phase, since a page handler may disable it on the way: New Chat
  // disables itself once the empty chat it opened is showing.
  let pressed: Element | null = null;
  document.addEventListener("click", (event) => {
    pressed = event.target instanceof Element ? event.target.closest(CONTROL) : null;
  }, { capture: true, signal });
  document.addEventListener("click", (event) => {
    const control = pressed;
    pressed = null;
    const step = tour.current;
    if (step?.dataset.advance !== "anchor" || !(event.target instanceof Element)) return;
    const anchor = event.target.closest(anchorSelector(step.dataset.anchor ?? ""));
    if (!anchor || !control || !anchor.contains(control)) return;
    const generation = tour.generation;
    window.setTimeout(() => {
      if (generation === tour.generation) next(tour);
    });
  }, { signal });
}

// anchorNow is the open step's anchor as the page has it now: the one the
// hole is on, or its replacement once a swap has taken that one out. The
// router settles on a swap before reanchor runs, so both the step's regions
// and its focus target look the anchor up afresh rather than wait for it.
function anchorNow(tour: Tour): HTMLElement | null {
  const step = tour.current;
  if (!step) return null;
  return tour.anchor?.isConnected === true ? tour.anchor : visibleAnchor(step.dataset.anchor ?? "");
}

// guard makes the tour a modal layer of the key router while the welcome
// dialog is open or the overlay is up, between steps included. An open
// step's regions are its anchor and the tour's root, which holds the popover;
// between steps there is only the root, so every key is outside the step --
// kept from the page, and Escape skips. The root is the tour's own: a key
// there is sealed at once, ahead of the page's capture listeners, and still
// presses the focused button. The anchor is page content the step borrows:
// its own listeners still run first -- the composer still types and sends --
// and a key nothing there claims goes no further. Focus that leaves the step
// comes back on the next key, to where it last was in the step, else to the
// step's focus target; a swap that takes the focused element along with the
// anchor puts focus on the replacement's. The root owns the layer, so a
// finished tour, whose root is gone, leaves none.
function guard(tour: Tour): void {
  layer({
    active: () => !tour.finished && (tour.welcome?.open === true || overlayShown(tour)),
    escape: () => {
      finish(tour, "skipped");
      return true;
    },
    id: "tour",
    initial: () => {
      const step = tour.current;
      const anchor = anchorNow(tour);
      return step && anchor ? focusTarget(step, anchor) : null;
    },
    own: () => [tour.root],
    owner: tour.root,
    regions: () => (tour.current ? [anchorNow(tour), tour.root] : [tour.root]),
  });
}

function start(root: HTMLElement): void {
  const steps = Array.from(root.querySelectorAll<HTMLElement>(STEP));
  const saved = root.dataset.step ?? WELCOME_STEP;
  const tour: Tour = {
    aborted: new AbortController(),
    anchor: null,
    current: null,
    dim: root.querySelector<SVGPathElement>(DIM),
    finished: false,
    frame: 0,
    generation: 0,
    index: steps.findIndex((step) => step.dataset.name === saved),
    observer: new MutationObserver(() => reanchor(tour)),
    overlay: root.querySelector<SVGSVGElement>(OVERLAY),
    pad: 0,
    ready: new MutationObserver(constVoid),
    resized: new ResizeObserver(() => drawHole(tour)),
    root,
    state: {},
    steps,
    welcome: root.querySelector<HTMLDialogElement>(WELCOME),
  };
  wire(tour);
  guard(tour);
  // Below docked the sidebar the first steps point into is an off-canvas
  // drawer, so the tour waits there rather than point at nothing.
  const docked = dockedQuery();
  docked.addEventListener("change", () => (docked.matches ? resume(tour) : pause(tour)), {
    signal: tour.aborted.signal,
  });
  if (docked.matches) resume(tour);
}

// The tour needs the top layer for its popovers and checkVisibility to find
// its anchors; a browser missing either never starts it. Nor does a browser
// under automation, such as an agent driving the owner's session: the modal
// welcome dialog would block every control it clicks, and whatever it pressed
// would be saved as the owner's own run through the tour.
function boot(): void {
  const root = document.querySelector<HTMLElement>(ROOT);
  if (root && !navigator.webdriver && supportsTopLayer() && "checkVisibility" in Element.prototype) start(root);
}
// Page modules install the listeners that reveal a step's anchor. Wait for
// those deferred modules before the first step dispatches tour:reveal.
if (document.readyState === "complete") boot();
else document.addEventListener("DOMContentLoaded", boot, { once: true });
