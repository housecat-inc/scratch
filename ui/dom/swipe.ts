// Swiping a list row sideways to uncover its actions, on a touch screen: no
// hover and a coarse pointer. A pointer screen reveals the same actions on hover, so
// nothing here runs while that query does not match, and the CSS that slides
// the row is gated on the same one (the touch variant, ui/css/theme.css). A
// screen with no pointer at all can neither hover nor swipe, and keeps the
// actions on show in the row.
//
// A row is any [data-swipe] element; its trays are the parts marked
// data-swipe-part=tray, data-swipe-side left (the default, uncovered by a
// swipe left) or right (by a swipe right). The state lives on the row, where the row's and the
// tray's classes read it: data-swipe is closed, open or armed, --swipe-x is
// how far the content has slid (negative left, positive right), and data-swipe-dragging is
// set while a finger is moving it, so the content tracks the finger rather
// than easing after it. Nothing here knows what the actions do. A full swipe
// arms the tray's [data-swipe-commit] button and, released, clicks it: a real
// click, so whatever handles a tap on that button -- an optimistic update, a
// toast, an Undo -- handles the swipe the same way.
//
// The pan is ui/dom/pan.ts's. This module claims a pan that starts on a row,
// and pan.ts hands it over once it has travelled past the slop more sideways
// than vertical: leftward on a closed row, rightward too where it has a right
// tray with an action in it, and either way on an open one.
// Otherwise the pan is the browser's scroll or pull to refresh's, so a pan is
// never two of them. The row's touch-action is pan-y, so the browser keeps
// vertical scrolling and leaves a sideways pan to the pointer events.
// pan.ts swallows the click that ends a swipe, on that row alone.
//
// A closed row's tray is clipped out of sight but stays in the tab order and
// the accessibility tree. Focus arriving in it -- a keyboard, a switch, a
// screen reader -- opens the row so the focused action shows, and focus
// leaving the row shuts it again. A screen reader's activation is a click on
// the button, clipped or not, so it runs the action as a tap would.
//
// The thresholds are the row's data-swipe-open-at and data-swipe-commit-at,
// whole percents read at the start of each drag, so a call site can tune
// them by prop; a row without data-swipe-open-at opens where its first action
// and release cue are uncovered (ui/dom/swipemotion.ts). CommitAt is a
// percent of the viewport, or of the row where the row is narrower, so a row
// beside a sidebar can still be swiped far enough to arm. ?swipe=open,commit[,slop] on any page -- whole percents, and
// a slop in px -- stores an override in localStorage that wins over every
// row's until ?swipe=reset, for tuning on a device without a deploy; a chip
// in the corner says it is on.

import { intAttr, leaves } from "./dom";
import { forgetStorage, readStorage, writeStorage } from "./failures";
import { type Claim, type PanAt, reducedMotion, register } from "./pan";
import { armAt, parseSwipeOverride, readSwipeOverride, rest, slideX, type SwipeOverride } from "./panrules";
import { openRowActions } from "./rowmore";
import { measureSwipeMotion, paintSwipeMotion, type SwipeMotion } from "./swipemotion";

const ROW = "[data-swipe]";
const TRAY = "[data-swipe-part=tray]";
const COMMIT = "[data-swipe-commit]";
const STORE = "hcSwipeThresholds";
const CHIP = "swipe-override-chip";

// The chip's classes are written out whole so Tailwind finds them here.
const CHIP_CLASS =
  "pointer-events-none fixed bottom-[calc(var(--bottombar-height)+0.5rem)] left-2 z-50 rounded-full bg-foreground/80 px-2 py-0.5 type-caption-100 text-background tabular-nums docked:bottom-2";

// Percent of the viewport, or of the row where it is narrower, matching
// swiperow.CommitAt.
const COMMIT_AT = 50;

// How long the slide animates (swiperow's duration-250), for the commit that
// waits for the row to finish leaving before it clicks.
const SLIDE_MS = 250;

// How long a row whose action took it off the list is left as it is before it
// is reset, so it is not seen sliding back while it fades. An Undo that puts it
// back gets it closed.
const SETTLE_MS = 300;

// Over-dragging a row with nothing to commit moves it this fraction of the
// finger's travel past the tray, so it reads as the end of the road.
const RESISTANCE = 0.3;

type Side = "left" | "right";

// Trays are a row's, by side, looked up once a drag starts rather than on
// every move.
type Trays = Record<Side, HTMLElement | null>;

type Drag = {
  ready: boolean;
  // When the touch went down, which a flick's time counts from
  // (panrules.rest), rather than from the slop the pan was decided at.
  downAt: number;
  side: Side;
  direction: number;
  armAt: number;
  base: number;
  commit: HTMLElement | null;
  openAt: number;
  rest: string;
  row: HTMLElement;
  trays: Trays;
  trayWidth: number;
  width: number;
  x: number;
};

const touch = window.matchMedia("(hover: none) and (pointer: coarse)");

let drag: Drag | null = null;
let open: HTMLElement | null = null;
const motions = new WeakMap<HTMLElement, SwipeMotion>();

// The stored override, read once when the page loads: only ?swipe= on a load
// changes it.
let override: SwipeOverride | null = null;

// Rows waiting out a commit's slide before their action is clicked; a finger
// landing on one then is ignored rather than dragging it back.
const committed = new WeakSet<HTMLElement>();

// When the touch behind the last claim went down: pan.ts claims on
// pointerdown, and only that pan's start can follow.
let claimedAt = 0;

// traysOf finds row's own trays, not those of a row nested in it, the first
// of each side.
function traysOf(row: Element): Trays {
  const out: Trays = { left: null, right: null };
  for (const tray of Array.from(row.querySelectorAll<HTMLElement>(TRAY))) {
    if (tray.closest(ROW) !== row) continue;
    const side = tray.getAttribute("data-swipe-side") || "left";
    if ((side === "left" || side === "right") && !out[side]) out[side] = tray;
  }
  return out;
}

function trayOf(row: Element, side: Side = "left"): HTMLElement | null {
  return traysOf(row)[side];
}

function slide(row: HTMLElement, x: number, trays: Trays = traysOf(row)): void {
  if (x === 0) row.style.removeProperty("--swipe-x");
  else row.style.setProperty("--swipe-x", x + "px");
  for (const side of ["left", "right"] as const) {
    const tray = trays[side];
    if (!tray) continue;
    const motion = motions.get(tray);
    if (motion) paintSwipeMotion(tray, motion, Math.max(0, side === "left" ? -x : x));
  }
}

// still applies a state with the transitions off, for a change nobody should
// see move.
function still(row: HTMLElement, apply: () => void): void {
  row.setAttribute("data-swipe-dragging", "");
  apply();
  void row.offsetWidth;
  row.removeAttribute("data-swipe-dragging");
}

// close slides row shut.
function close(row: HTMLElement, instant = false): void {
  if (open === row) open = null;
  const apply = () => {
    row.setAttribute("data-swipe", "closed");
    row.removeAttribute("data-swipe-active");
    slide(row, 0);
  };
  if (instant) still(row, apply);
  else apply();
}

// keyboardIn says a keyboard's focus is in row. A focus a tap left on a button
// does not count: only a keyboard user needs the row kept open to see it.
function keyboardIn(row: Element): boolean {
  const focused = document.activeElement;
  return focused instanceof Element && row.contains(focused) && focused.matches(":focus-visible");
}

function openRow(row: HTMLElement, trayWidth: number, side: "left" | "right" = "left"): void {
  if (open && open !== row) close(open);
  open = row;
  const tray = trayOf(row, side);
  if (tray && !motions.has(tray)) motions.set(tray, measureSwipeMotion(row, tray, side));
  if (tray) trayWidth = motions.get(tray)?.width ?? trayWidth;
  row.setAttribute("data-swipe-active", "");
  row.setAttribute("data-swipe", "open");
  slide(row, side === "left" ? -trayWidth : trayWidth);
}

// menuOpenIn reports whether a menu in the row (Remind) is showing: the
// shared menu marks its trigger while its popup is open.
function menuOpenIn(row: HTMLElement): boolean {
  return !!row.querySelector("[data-slot=menu-trigger][data-popup-open]");
}

// settle tidies a row after one of its tray's actions ran: shut if it is
// still on the list, and reset without moving if the action took it off, so
// an Undo that puts it back finds it closed. A menu the action opened is
// anchored to the tray, and a keyboard's focus is still on the action, so
// either keeps the row open until a tap elsewhere or focus leaving shuts it.
function settle(row: HTMLElement): void {
  setTimeout(() => {
    if (row.hasAttribute("data-done-pending") || !row.isConnected) {
      if (open === row) open = null;
      setTimeout(() => close(row, true), SETTLE_MS);
    } else if (open !== row || !(menuOpenIn(row) || keyboardIn(row))) {
      close(row);
    }
  }, 0);
}

// commit slides the row off and then clicks its commit button. A list
// refreshed in the meantime may have swapped the button for a new one, which
// is looked up again through the row's data-id; with none to click, the row
// just shuts. The click has no pointer behind it, so pan.ts lets it through.
function commit(row: HTMLElement, button: HTMLElement, width: number, side: "left" | "right"): void {
  if (open === row) open = null;
  committed.add(row);
  row.setAttribute("data-swipe", "armed");
  slide(row, side === "left" ? -width : width);
  setTimeout(() => {
    let target: HTMLElement | null = button;
    if (!target.isConnected) {
      const id = row.getAttribute("data-id");
      target = id
        ? document.querySelector<HTMLElement>(
          `${ROW}[data-id="${CSS.escape(id)}"] ${TRAY}[data-swipe-side="${side}"] ${COMMIT}`,
        )
        : null;
    }
    target?.click();
    setTimeout(() => {
      committed.delete(row);
      close(row, !row.isConnected || row.hasAttribute("data-done-pending"));
    }, target ? SETTLE_MS : 0);
  }, reducedMotion() ? 0 : SLIDE_MS);
}

function claim(target: Element): Claim | null {
  if (!touch.matches) return null;
  const row = target.closest<HTMLElement>(ROW);
  if (!row || committed.has(row)) return null;
  const trays = traysOf(row);
  const tray = trays.left;
  if (!tray || tray.getBoundingClientRect().width === 0 || row.getBoundingClientRect().width === 0) return null;
  const c: Claim = {
    el: row,
    kind: "row",
    open: row.getAttribute("data-swipe") === "open",
    right: !!trays.right?.querySelector("button, a"),
  };
  if (override?.slopPx !== undefined) c.slopPx = override.slopPx;
  claimedAt = performance.now();
  return c;
}

// start measures the row and reads its thresholds, with any stored override
// on top, as the pan becomes its.
function start(row: HTMLElement, at: PanAt): void {
  const current = parseFloat(row.style.getPropertyValue("--swipe-x")) || 0;
  const side = (current || at.dx) > 0 ? "right" : "left";
  const direction = side === "right" ? 1 : -1;
  const trays = traysOf(row);
  const tray = trays[side];
  if (!tray) return;
  const motion = measureSwipeMotion(row, tray, side);
  motions.set(tray, motion);
  const trayWidth = motion.width;
  const width = row.getBoundingClientRect().width;
  const state = row.getAttribute("data-swipe") || "closed";
  const base = state === "open" ? -trayWidth : 0;
  // Tuning never arms the commit short of half the screen, or of a narrower
  // row.
  const commitAt = Math.max(0.5, (override?.commitAt ?? intAttr(row, "data-swipe-commit-at", COMMIT_AT)) / 100);
  const commitPastPx = Math.max(1, intAttr(row, "data-swipe-commit-past-px", motion.size));
  const openAt = override?.openAt ?? intAttr(row, "data-swipe-open-at", 0);
  const viewport = window.visualViewport?.width ?? document.documentElement.clientWidth;
  drag = {
    ready: state === "open",
    downAt: claimedAt,
    direction,
    side,
    // The drag never slides the row past its own width, so CommitAt of a
    // viewport wider than the row could never be reached.
    armAt: armAt(commitAt, Math.min(viewport, width), motion.fullWidth, commitPastPx),
    base,
    // A busy commit button -- its last action still settling -- is not one;
    // the row only opens.
    commit: tray.querySelector<HTMLElement>(
      `${COMMIT}:not(:disabled):not([aria-disabled=true]):not([data-optimistic-busy=true])`,
    ),
    openAt: openAt > 0 ? openAt / 100 : motion.openAt / trayWidth,
    rest: state,
    row,
    trays,
    trayWidth,
    width,
    x: base,
  };
  row.setAttribute("data-swipe-active", "");
  row.setAttribute("data-swipe-dragging", "");
}

// buzz vibrates for ms where the browser can: Safari has no navigator.vibrate,
// which TypeScript's DOM types leave out.
export function buzz(ms: number): void {
  (navigator.vibrate as Navigator["vibrate"] | undefined)?.(ms);
}

function move(row: HTMLElement, at: PanAt): void {
  const d = drag;
  if (!d || d.row !== row) return;
  d.x = slideX({
    base: d.base,
    commit: !!d.commit,
    dx: -at.dx * d.direction,
    resistance: RESISTANCE,
    trayWidth: d.trayWidth,
    width: d.width,
  });
  slide(row, -d.x * d.direction, d.trays);
  const ready = -d.x >= d.openAt * d.trayWidth;
  const wasReady = d.ready;
  d.ready = ready;
  const armed = !!d.commit && -d.x > d.armAt;
  const state = row.getAttribute("data-swipe");
  if (armed && state !== "armed") {
    row.setAttribute("data-swipe", "armed");
    buzz(10);
  } else if (!armed && state === "armed") {
    row.setAttribute("data-swipe", d.rest);
  }
  if (ready && !wasReady && !armed) buzz(6);
  const outcome = releaseOutcome(d, at);
  const tray = d.trays[d.side];
  if (tray) {
    const towardOpen = outcome !== "closed";
    const direction = (d.side === "left") === towardOpen ? "1" : "-1";
    if (tray.style.getPropertyValue("--swipe-chevron-direction") !== direction) {
      tray.style.setProperty("--swipe-chevron-direction", direction);
    }
  }
}

// releaseOutcome is where the row comes to rest if let go now
// (panrules.rest). Its speed is taken along the way the tray opens, negative
// toward open, so only a flick toward open is one, and its time from when the
// touch went down.
function releaseOutcome(d: Drag, release: PanAt): ReturnType<typeof rest> {
  return rest({
    armed: !!d.commit && d.row.getAttribute("data-swipe") === "armed",
    base: d.base,
    elapsedMs: performance.now() - d.downAt,
    openAt: d.openAt,
    trayWidth: d.trayWidth,
    x: d.x,
    v: -release.v * d.direction,
  });
}

function end(row: HTMLElement, release: PanAt): void {
  const d = drag;
  drag = null;
  if (!d || d.row !== row) return;
  row.removeAttribute("data-swipe-dragging");
  const at = releaseOutcome(d, release);
  if (at === "commit" && d.commit) commit(row, d.commit, d.width, d.side);
  else if (at === "open") openRow(row, d.trayWidth, d.side);
  else close(row);
}

// A pan the browser took over -- a scroll, a second finger -- puts the row
// back as it was.
function cancel(row: HTMLElement): void {
  const d = drag;
  drag = null;
  if (!d || d.row !== row) return;
  row.removeAttribute("data-swipe-dragging");
  if (d.base < 0) openRow(row, d.trayWidth, d.side);
  else close(row);
}

register({ cancel, claim, end, hold: openRowActions, move, start });

// A touch anywhere but the open row shuts it.
function onPointerDown(e: PointerEvent): void {
  if (!touch.matches || !e.isPrimary || e.button !== 0) return;
  if (open && !open.isConnected) open = null;
  if (open && !(e.target instanceof Node && open.contains(e.target))) close(open);
}

// Capture phase on the document: a tap on an open row shuts it rather than
// opening the thread under it, and a tap on its tray runs the action and then
// shuts it. Inline scripts that register their own document capture listeners
// before this deferred module still see such a click -- stopPropagation does
// not reach listeners on the same node -- and so does rowmore.ts, whose More
// button on an open row therefore still opens its card as the row shuts.
function onClick(e: MouseEvent): void {
  const row = open;
  if (!row || !(e.target instanceof Node) || !row.contains(e.target)) return;
  const tray = e.target instanceof Element ? e.target.closest(TRAY) : null;
  if (tray && row.contains(tray)) {
    settle(row);
    return;
  }
  // A keyboard's or a script's activation is not a tap: it goes through, and
  // the row shuts behind it.
  if (e.detail === 0) {
    close(row);
    return;
  }
  e.preventDefault();
  e.stopPropagation();
  close(row);
}

// Focus arriving in a closed row's tray opens the row, so the action it
// lands on is in sight.
function onFocusIn(e: FocusEvent): void {
  if (!touch.matches || !(e.target instanceof Element)) return;
  const tray = e.target.closest<HTMLElement>(TRAY);
  const row = tray?.closest<HTMLElement>(ROW);
  if (!tray || !row || !row.matches(ROW) || committed.has(row)) return;
  if (row.getAttribute("data-swipe") !== "closed" || drag?.row === row) return;
  const trayWidth = tray.getBoundingClientRect().width;
  if (!trayWidth) return;
  // Measured afresh, since the tray may have been redrawn or the screen
  // turned since its last drag, and a closed row's cells are at rest.
  const side = tray.getAttribute("data-swipe-side") === "right" ? "right" : "left";
  motions.set(tray, measureSwipeMotion(row, tray, side));
  openRow(row, trayWidth, side);
}

// Focus leaving an open row for somewhere outside it shuts it.
function onFocusOut(e: FocusEvent): void {
  if (!touch.matches || !(e.target instanceof Element)) return;
  const row = e.target.closest<HTMLElement>(ROW);
  if (!row || row !== open || drag?.row === row) return;
  if (!leaves(row, e.relatedTarget)) return;
  close(row);
}

// Scrolling whatever holds the open row shuts it -- unless a keyboard's focus
// is in it, since focus scrolls what it lands on into view, and focus leaving
// shuts it instead.
function onScroll(e: Event): void {
  const row = open;
  if (!row || keyboardIn(row)) return;
  const target = e.target;
  if (target === document || (target instanceof Node && target !== row && target.contains(row))) close(row);
}

// readOverride takes ?swipe= off the URL, so a shared link never carries it,
// and stores or clears what it said.
function readOverride(): void {
  const url = new URL(location.href);
  const value = url.searchParams.get("swipe");
  if (value === null) return;
  // Storage can be off, in a private window; the URL is still tidied.
  if (value === "reset") {
    forgetStorage("local", STORE);
  } else {
    const parsed = parseSwipeOverride(value);
    if (parsed) writeStorage("local", STORE, JSON.stringify(parsed));
  }
  url.searchParams.delete("swipe");
  history.replaceState(history.state, "", url.pathname + url.search + url.hash);
}

function stored(): SwipeOverride | null {
  return readSwipeOverride(readStorage("local", STORE));
}

function showChip(t: SwipeOverride | null): void {
  document.querySelectorAll(`[data-slot=${CHIP}]`).forEach((el) => el.remove());
  if (!t) return;
  const chip = document.createElement("div");
  chip.setAttribute("data-slot", CHIP);
  chip.setAttribute("aria-hidden", "true");
  chip.className = CHIP_CLASS;
  chip.textContent = "swipe " + t.openAt + " / " + t.commitAt + (t.slopPx !== undefined ? " / " + t.slopPx + "px" : "");
  document.body.appendChild(chip);
}

function begin(): void {
  readOverride();
  override = stored();
  showChip(override);
}

document.addEventListener("pointerdown", onPointerDown, { passive: true });
document.addEventListener("click", onClick, true);
document.addEventListener("scroll", onScroll, { capture: true, passive: true });
document.addEventListener("focusin", onFocusIn);
document.addEventListener("focusout", onFocusOut);

// A mouse plugged into a tablet brings hover back, and the hover cluster with
// it; a row left open would sit slid under it.
touch.addEventListener("change", () => {
  if (drag) close(drag.row, true);
  drag = null;
  document.querySelectorAll<HTMLElement>(`${ROW}:not([data-swipe=closed])`).forEach((row) => close(row, true));
});

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", begin, { once: true });
else begin();
