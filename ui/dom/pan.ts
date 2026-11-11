// One pointer pan at a time, from pointerdown to release, decided once as
// the bottom sheet's drag (ui/components/drawer/sheet.ts), a swipe row's
// (ui/dom/swipe.ts), pull to refresh's (MailPullRefreshScript, through
// window.HousecatPan), or else the browser's own scroll. Each of those
// registers an owner here.
//
// On pointerdown every owner says whether it could take a pan starting where
// the finger landed: its claim. The pan is then one record of the claims,
// the start and the moves, and nothing is written while it is short of the
// slop. Each claim asks for its own slopPx, else its element's
// data-pan-slop-px, else panrules.SLOP_PX, and the pan's slop is the largest
// of those (panrules.slop), so no candidate is decided sooner than it asked:
// a row's slop holds at the top of the list too, where a pull is also a
// candidate. A tap that wandered that little is still a tap, and its click
// goes through. The first move past the slop locks the pan to
// an axis and hands it to the one claim that takes that direction
// (panrules.decide), or to nobody, and the browser has it. The owner then
// has the pointer captured on its element and hears start, every move, and
// end or cancel, each with the travel from the start and the velocity along
// the axis from @use-gesture's pointer engine, including its release-pause handling.
//
// The browser has to be kept from claiming a pan as a scroll before the pan
// is decided. touch-action does that where it can: a row's pan-y, the
// sheet's pan-x pan-down. Where it cannot, which is Safari for the sheet and
// both browsers for a list at its top, a touchmove listener that is not
// passive cancels the moves. It cancels every move from the lock once an
// owner has the pan, and, for a pan that could be a pull or the sheet's drag,
// every move more down than sideways from the very first (panrules.holdsDown),
// because a list at its top, or the sheet's content in Safari, otherwise
// starts the browser's overscroll before the slop is reached. That listener is
// bound only where it is needed (cancelTouchOn): on each sheet, and on the
// document once pull to refresh registers. A touch anywhere else is not held
// up waiting on it.
//
// A pan that starts in a field or a popover is nobody's: it selects or
// scrolls there, as the browser has it.
//
// The click that ends a pan lands on what the finger started on, or on the
// owner's element, which holds the pointer. It is swallowed when it falls
// inside that element within CLICK_AFTER_PAN_MS of the release, so a drag
// from a link does not follow it. The next pointerdown, or any other click,
// ends that. A click with no pointer behind it (detail 0) always goes
// through: a keyboard's, or a script's, such as a swipe's commit pressing its
// own button. The listener is on the window in the capture phase and stops
// the click outright, so it runs ahead of every document listener: a
// component's in ui.js, and an inline script's bound before ui.js ran, such
// as the optimistic archive's, which would otherwise act on the click first.
//
// A second finger ends the pan as a cancel, so a pinch is the browser's.

import { DragGesture } from "@use-gesture/vanilla";
import { intAttr } from "./dom";
import { whileAttached } from "./failures";
import { type Axis, type Candidate, CLICK_AFTER_PAN_MS, decide, holdsDown, slop, SLOP_PX } from "./panrules";

// Claim is an owner's answer to a pointerdown: the element it would move,
// what the pan would be to it, and the slop it asks for when that is not its
// element's data-pan-slop-px or SLOP_PX.
export type Claim = Candidate & { el: HTMLElement; slopPx?: number };

// PanAt is a pan as its owner hears it: the travel from where it started,
// and the velocity along its axis over the last moves, in px/ms.
export type PanAt = { dx: number; dy: number; v: number };

export type PanOwner = {
  cancel: (el: HTMLElement) => void;
  claim: (target: Element, event: PointerEvent) => Claim | null;
  end: (el: HTMLElement, at: PanAt) => void;
  hold?: (el: HTMLElement) => void;
  move: (el: HTMLElement, at: PanAt) => void;
  start: (el: HTMLElement, at: PanAt) => void;
};

type Held = { claim: Claim; owner: PanOwner };

type Pan = {
  axis: Axis;
  held: Held[];
  owner: Held | null;
  holding: Held | null;
  holdTimer: ReturnType<typeof setTimeout> | null;
  pointer: number;
  v: number;
  slopPx: number;
  x0: number;
  y0: number;
};

declare global {
  interface Window {
    HousecatPan: {
      cancelTouchOn: typeof cancelTouchOn;
      register: typeof register;
    };
  }
}

// IGNORE is where a pan is never decided.
const IGNORE = "input, textarea, select, [contenteditable], [popover]";
// HOLD_IGNORE are the controls a hold leaves to their own press.
const HOLD_IGNORE = "button, [role=button], [role=checkbox]";
// HOLD_EXEMPT is everywhere a long press stays the browser's: where no pan is
// decided, and on a control of its own. ui/dom/rowmore.ts leaves the same to
// the browser's context menu, so Android's long press and the hold agree.
export const HOLD_EXEMPT = `${IGNORE}, ${HOLD_IGNORE}`;

const owners: PanOwner[] = [];
const cancelling = new WeakSet<GlobalEventHandlers>();

let pan: Pan | null = null;
// The element the last pan moved and when it let go, for the click that
// ends that pan; null once a click or a pointerdown has come.
let swallow: { at: number; el: Element; anywhere?: boolean } | null = null;
let reduce: MediaQueryList | null = null;

export function register(owner: PanOwner): void {
  if (!owners.includes(owner)) owners.push(owner);
}

// reducedMotion is prefers-reduced-motion, for an owner deciding whether a
// release animates. The query is made on first use and read live.
export function reducedMotion(): boolean {
  reduce ??= window.matchMedia("(prefers-reduced-motion: reduce)");
  return reduce.matches;
}

// cancelTouchOn binds the touchmove canceller on target, once.
export function cancelTouchOn(target: GlobalEventHandlers): void {
  if (cancelling.has(target)) return;
  cancelling.add(target);
  target.addEventListener("touchmove", cancelTouch, { passive: false });
}

function travel(p: Pan, x: number, y: number): PanAt {
  return { dx: x - p.x0, dy: y - p.y0, v: p.v };
}

// drop ends the pan in progress, if any, as a cancel to its owner.
function drop(): void {
  const p = pan;
  pan = null;
  if (p) clearHold(p);
  if (p?.owner) p.owner.owner.cancel(p.owner.claim.el);
}

function clearHold(p: Pan): void {
  if (p.holdTimer !== null) clearTimeout(p.holdTimer);
  p.holdTimer = null;
}

// lock decides p at (x, y), once it is past the slop. A hold is a touch held
// still, so one that has wandered a tap's slop is no hold, even where the pan
// waits for a larger slop.
function lock(p: Pan, x: number, y: number): void {
  if (Math.abs(x - p.x0) >= SLOP_PX || Math.abs(y - p.y0) >= SLOP_PX) clearHold(p);
  const d = decide(x - p.x0, y - p.y0, p.slopPx, p.held.map((h) => h.claim));
  if (d.kind === "pending") return;
  clearHold(p);
  const held = d.kind === "owner" ? p.held[d.index] : undefined;
  if (d.kind === "browser" || !held) {
    pan = null;
    return;
  }
  p.owner = held;
  p.axis = d.axis;
  // A pointer the browser already let go of has nothing to capture; the
  // document listeners still see it.
  const captured = held.claim.el;
  whileAttached("pan.capture", () => captured.setPointerCapture(p.pointer));
  held.owner.start(held.claim.el, travel(p, x, y));
}

function onDown(e: PointerEvent): void {
  if (!e.isPrimary) {
    drop();
    return;
  }
  swallow = null;
  drop();
  const target = e.target;
  if (e.button !== 0 || !(target instanceof Element) || target.closest(IGNORE)) return;
  const held: Held[] = [];
  for (const owner of owners) {
    const claim = owner.claim(target, e);
    if (claim) held.push({ claim, owner });
  }
  if (!held.length) return;
  pan = {
    axis: "y",
    held,
    owner: null,
    holding: null,
    holdTimer: null,
    pointer: e.pointerId,
    v: 0,
    slopPx: slop(held.map(({ claim }) => claim.slopPx ?? intAttr(claim.el, "data-pan-slop-px", SLOP_PX))),
    x0: e.clientX,
    y0: e.clientY,
  };
  const p = pan;
  // A touch held still on an owner with a hold is the hold's, unless it is on
  // a control of its own. A link is no such control: a mail row's sender,
  // subject and preview are its links, and the row is what is held.
  const hold = held.find(h => h.owner.hold !== undefined);
  if (hold && e.pointerType === "touch" && !target.closest(HOLD_IGNORE)) {
    p.holdTimer = setTimeout(() => {
      p.holdTimer = null;
      if (pan !== p || p.owner || !hold.claim.el.isConnected) return;
      p.holding = hold;
      try {
        hold.claim.el.setPointerCapture(p.pointer);
      } catch {
        drop();
        return;
      }
      hold.owner.hold?.(hold.claim.el);
    }, 500);
  }
}

function onMove(e: PointerEvent): void {
  const p = pan;
  if (!p || e.pointerId !== p.pointer) return;
  if (p.holding) return;
  if (!p.owner) lock(p, e.clientX, e.clientY);
  const held = p.owner;
  if (pan !== p || !held) return;
  held.owner.move(held.claim.el, travel(p, e.clientX, e.clientY));
}

function onUp(e: PointerEvent): void {
  const p = pan;
  if (!p || e.pointerId !== p.pointer) return;
  pan = null;
  clearHold(p);
  if (p.holding) {
    swallow = { at: performance.now(), el: p.holding.claim.el, anywhere: true };
    return;
  }
  const held = p.owner;
  if (!held) return;
  swallow = { at: performance.now(), el: held.claim.el };
  held.owner.end(held.claim.el, travel(p, e.clientX, e.clientY));
}

// A pan the browser took after all -- a scroll -- puts back whatever its
// owner moved.
function onCancel(e: PointerEvent): void {
  const p = pan;
  if (!p || e.pointerId !== p.pointer) return;
  drop();
}

function cancelTouch(e: TouchEvent): void {
  const p = pan;
  const touch = e.touches[0] as Touch | undefined;
  if (!p || !touch || e.touches.length !== 1 || !e.cancelable) return;
  if (p.holding) {
    e.preventDefault();
    return;
  }
  if (!p.owner) lock(p, touch.clientX, touch.clientY);
  if (pan !== p) return;
  if (p.owner) {
    e.preventDefault();
    return;
  }
  const down = p.held.some(({ claim }) => claim.kind === "sheet" || (claim.kind === "pull" && claim.el.scrollTop <= 0));
  if (down && holdsDown(touch.clientX - p.x0, touch.clientY - p.y0)) e.preventDefault();
}

function onClick(e: MouseEvent): void {
  const s = swallow;
  if (!s || e.detail === 0) return;
  swallow = null;
  if (performance.now() - s.at > CLICK_AFTER_PAN_MS) return;
  if (s.anywhere !== true && (!(e.target instanceof Node) || !s.el.contains(e.target))) return;
  e.preventDefault();
  e.stopImmediatePropagation();
}

document.addEventListener("pointerdown", onDown, { capture: true, passive: true });
// The drag hears pointer events only: keys is off below, and touch and mouse,
// which would swap them for touch or mouse events, are left off. So every
// event is a PointerEvent, which the type parameter tells the package's types.
new DragGesture<PointerEvent>(document.documentElement, (state) => {
  const e = state.event;
  const p = pan;
  if (!p || e.pointerId !== p.pointer) return;
  const index = p.owner ? (p.axis === "x" ? 0 : 1) : (Math.abs(e.clientX - p.x0) > Math.abs(e.clientY - p.y0) ? 0 : 1);
  p.v = state.velocity[index] * state.direction[index];
  if (e.type === "pointercancel" || state.canceled) onCancel(e);
  else if (state.last) onUp(e);
  else if (e.type === "pointermove") onMove(e);
}, {
  threshold: 0,
  filterTaps: false,
  eventOptions: { capture: true, passive: true },
  pointer: { capture: false, keys: false },
});
window.addEventListener("click", onClick, true);

window.HousecatPan = { cancelTouchOn, register };
