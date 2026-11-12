// The decisions behind a pointer pan (ui/dom/pan.ts) and behind the two
// owners that move something with one -- the bottom sheet
// (ui/components/drawer/sheet.ts) and a swipe row (ui/dom/swipe.ts) -- as
// functions of numbers. Each owner keeps its own policy (how far, how fast,
// how stiff) and passes it in. Nothing here reaches the window or the
// document, so Vitest runs these in Node.

import * as Option from "effect/Option";
import * as Schema from "effect/Schema";

import { clamp } from "./dom";

export type PanKind = "hold" | "pull" | "row" | "sheet";

// Candidate is what a pan could become, as the owner that would take it
// describes it: grabber for a pan starting on the sheet's grabber, open for a
// row already slid open.
export type Candidate = { grabber?: boolean; kind: PanKind; open?: boolean; right?: boolean };

export type Axis = "x" | "y";

export type Decision = { kind: "browser" } | { kind: "pending" } | { axis: Axis; index: number; kind: "owner" };

// SLOP_PX is how far a pan travels, along either axis, before it is decided:
// the tap slop both browsers use (Chrome's is about 8px, iOS's about 10).
// Inside it a pan is still a tap, and the browser clicks what it was on.
export const SLOP_PX = 10;

// slop is how far a pan with these candidates travels before it is decided,
// given the slop each asks for: the largest, so that no candidate is decided
// sooner than it asked. An ask of zero or less is SLOP_PX's, as swiperow.Props
// reads zero, so no pan is decided before it has moved. A pan with nothing
// asked has SLOP_PX.
export function slop(asks: readonly number[]): number {
  return asks.length ? Math.max(...asks.map((a) => (a > 0 ? a : SLOP_PX))) : SLOP_PX;
}

// CLICK_AFTER_PAN_MS is how long after a pan a click is still taken for the
// one that ends it.
export const CLICK_AFTER_PAN_MS = 400;

const PENDING: Decision = { kind: "pending" };
const BROWSER: Decision = { kind: "browser" };

// decide settles a pan that has travelled (dx, dy) from where it started. It
// is still a tap short of slopPx along both axes. Past that it locks to its
// longer axis, a tie counting as vertical, and goes to the first candidate
// that takes that direction, or else to the browser.
export function decide(dx: number, dy: number, slopPx: number, candidates: readonly Candidate[]): Decision {
  if (Math.abs(dx) < slopPx && Math.abs(dy) < slopPx) return PENDING;
  const vertical = Math.abs(dy) >= Math.abs(dx);
  const index = candidates.findIndex((c) => takes(c, dx, dy, vertical));
  if (index < 0) return BROWSER;
  return { axis: vertical ? "y" : "x", index, kind: "owner" };
}

// takes says whether a candidate owns a pan going this way. The sheet takes
// a pan down, or up or down on its grabber. A row takes one leftward, or
// either way once it is open. A pull takes one down. A hold -- something
// only a long press acts on, such as a sidebar row (ui/dom/navhold.ts) --
// takes none.
// oxlint-disable-next-line typescript/consistent-return -- the switch covers every kind, which noImplicitReturns checks
function takes(c: Candidate, dx: number, dy: number, vertical: boolean): boolean {
  switch (c.kind) {
    case "hold":
      return false;
    case "pull":
      return vertical && dy > 0;
    case "row":
      return !vertical && (dx < 0 || c.open === true || c.right === true);
    case "sheet":
      return vertical && (dy > 0 || c.grabber === true);
  }
}

// holdsDown says whether a touch move (dx, dy) from the start of an
// undecided pan that could be a pull or the sheet's drag is one the browser
// must not have: any move more down than sideways, from the first, since a
// list at its top, or the sheet's content in Safari, which reads no pan-down,
// starts the browser's own overscroll well inside the slop.
export function holdsDown(dx: number, dy: number): boolean {
  return dy > 0 && Math.abs(dx) <= dy;
}

// lift is how far a finger dy below where it started moves the sheet: one to
// one downward, and upward ever less, towards liftPx.
export function lift(dy: number, liftPx: number): number {
  if (dy >= 0) return dy;
  return -liftPx * (1 - Math.exp(dy / (liftPx * 3)));
}

// Release is a sheet let go y px below where it rests, of its height, moving
// down at v px/ms (up is negative).
export type Release = { height: number; v: number; y: number };

// shut says whether a released sheet closes. A fling, faster than fling
// either way, goes the way it was flung: down closes, up keeps it open,
// wherever it is let go. Anything slower closes past closeAt of its height.
export function shut(r: Release, closeAt: number, fling: number): boolean {
  if (r.v > fling) return true;
  if (r.v < -fling) return false;
  return r.y > r.height * closeAt;
}

export type FlingTiming = { maxMs: number; minMs: number; refPxPerMs: number; slideMs: number };

// flingMs is how long a fling's close slides: slideMs for the whole sheet at
// refPxPerMs, shorter in proportion for less of the sheet left and for a
// faster flick, and kept between minMs and maxMs.
export function flingMs(r: Release, timing: FlingTiming): number {
  const left = (r.height - r.y) / r.height;
  return clamp((timing.slideMs * left * timing.refPxPerMs) / r.v, timing.minMs, timing.maxMs);
}

// Slide is a row's drag: where it started (base, 0 or the open tray's
// negative width), the finger's travel, and the row's measures.
export type Slide = { base: number; commit: boolean; dx: number; resistance: number; trayWidth: number; width: number };

// slideX is where a row's content is drawn during a drag: never right of
// closed nor past the row's own width, and for a row with nothing to commit,
// only resistance times the finger's travel past the tray.
export function slideX(s: Slide): number {
  const x = clamp(s.base + s.dx, -s.width, 0);
  if (!s.commit && x < -s.trayWidth) return -s.trayWidth + (x + s.trayWidth) * s.resistance;
  return x;
}

// A default action needs both the viewport threshold and an extra pull
// beyond the full dragging tray, including its release cue.
export function armAt(commitAt: number, width: number, trayWidth: number, pastTrayPx: number): number {
  return Math.max(commitAt * width, trayWidth + pastTrayPx);
}

export type Rest = "closed" | "commit" | "open";

// RowRelease is a row let go at x, from base, armed or not, elapsedMs after
// the touch went down, moving at v px/ms along the way the tray opens
// (negative toward open).
export type RowRelease = {
  armed: boolean;
  base: number;
  elapsedMs?: number;
  openAt: number;
  trayWidth: number;
  x: number;
  v?: number;
};

// A flick is let go at least FLICK_PX out, and either fast toward open
// (FLICK_V) or quick -- within FLICK_MS of the touch going down -- and still
// moving toward open. A slow short drag, or one that paused before lifting,
// is no flick.
const FLICK_PX = 20;
const FLICK_MS = 250;
const FLICK_V = 0.5;

function flicked(r: RowRelease): boolean {
  const v = r.v ?? 0;
  return -r.x >= FLICK_PX && (v < -FLICK_V || ((r.elapsedMs ?? Infinity) <= FLICK_MS && v < 0));
}

// Flicks reveal actions even after crossing the commit threshold. A slow
// release commits only while armed; reversing an open tray closes it.
export function rest(r: RowRelease): Rest {
  if (r.base < 0 && r.x > r.base) return "closed";
  if (flicked(r)) return "open";
  if (r.armed) return "commit";
  if (-r.x >= r.openAt * r.trayWidth) return "open";
  return "closed";
}

// SwipeOverride is a device's own swipe thresholds (?swipe=, ui/dom/swipe.ts),
// as swipe.ts stores them: OpenAt and CommitAt in whole percents, and a slop
// in whole px.
const Percent = Schema.Int.check(Schema.isGreaterThan(0), Schema.isLessThan(100));
const SwipeOverride = Schema.Struct({
  commitAt: Percent,
  openAt: Percent,
  slopPx: Schema.optionalKey(Schema.Int.check(Schema.isGreaterThan(0))),
});
export type SwipeOverride = typeof SwipeOverride.Type;
const swipeOverride = Schema.decodeUnknownOption(SwipeOverride);
const storedSwipeOverride = Schema.decodeUnknownOption(Schema.fromJsonString(SwipeOverride));

// parseSwipeOverride reads ?swipe=open,commit[,slop].
export function parseSwipeOverride(value: string): SwipeOverride | null {
  const parts = value.split(",");
  if (parts.length < 2 || parts.length > 3) return null;
  const [openAt, commitAt, slopPx] = parts.map((p) => (p.trim() === "" ? NaN : Number(p)));
  return Option.getOrNull(swipeOverride(parts.length === 2 ? { commitAt, openAt } : { commitAt, openAt, slopPx }));
}

// readSwipeOverride reads the override as swipe.ts stores it. A stored value
// whose thresholds are not whole percents, or whose slop is not whole px, is
// ignored.
export function readSwipeOverride(raw: string | null): SwipeOverride | null {
  return raw ? Option.getOrNull(storedSwipeOverride(raw)) : null;
}
