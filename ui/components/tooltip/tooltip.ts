// The delegated script every tooltip on a page shares: one set of document
// listeners opens and closes any tooltip root, so a page with a hundred of
// them costs the same as a page with one.

import { leaves, supportsTopLayer } from "../../dom/dom";
import { whileAttached } from "../../dom/failures";
import { observe } from "../../dom/keys";
import { registerComponent } from "../../dom/lifecycle";
import { track, untrack } from "../../floating/floating";

const ROOT = "[data-slot=tooltip]";
const TRIGGER = "[data-slot=tooltip-trigger]";
const POSITIONER = "[data-slot=tooltip-positioner]";
const POPUP = "[data-slot=tooltip-popup]";

// Every tooltip waits the same OPEN_DELAY, so a pointer crossing a row of
// chips does not flash a label at each one. Keyboard focus never waits, and a
// tooltip opened within GROUP_TIMEOUT of another closing opens at once, which
// keeps a sweep along a dense row quick without a longer or shorter delay per
// call site.
const OPEN_DELAY = 700;
const GROUP_TIMEOUT = 400;
const ENDING_TIMEOUT = 300;

type Parts = {
  popup: HTMLElement;
  positioner: HTMLElement;
  trigger: HTMLElement;
};

type State = {
  endingTimer: number;
  frame: number;
  onEnd: ((event: TransitionEvent) => void) | null;
  open: boolean;
  openTimer: number;
  parts: Parts | null;
};

// A trigger and the root it belongs to, resolved together: every handler needs
// the trigger to tell a real boundary crossing from a move between its own
// children, and the root to act on.
type Hit = {
  root: Element;
  trigger: HTMLElement;
};

const states = new WeakMap<Element, State>();
const openRoots = new Set<Element>();

// Roots whose popup was empty when the pointer arrived, watched in case its
// text turns up while the pointer is still resting on the trigger.
const waiting = new WeakMap<Element, MutationObserver>();

// When a tooltip closed moments ago the next one opens with no delay, so
// sweeping along a row of icon buttons reads as one continuous hover.
let lastClosedAt = -Infinity;

function stateOf(root: Element): State {
  let st = states.get(root);
  if (!st) {
    st = { endingTimer: 0, frame: 0, onEnd: null, open: false, openTimer: 0, parts: null };
    states.set(root, st);
  }
  return st;
}

function partsOf(root: Element): Parts | null {
  const trigger = root.querySelector<HTMLElement>(TRIGGER);
  const positioner = root.querySelector<HTMLElement>(POSITIONER);
  const popup = positioner?.querySelector<HTMLElement>(POPUP) ?? null;
  if (!trigger || !positioner || !popup) return null;
  return { popup, positioner, trigger };
}

// A trigger that wraps an inline element has no box of its own to anchor to.
function anchorOf(trigger: HTMLElement): Element {
  const rect = trigger.getBoundingClientRect();
  if (rect.width || rect.height || !trigger.firstElementChild) return trigger;
  return trigger.firstElementChild;
}

function cancelEnding(st: State, parts: Parts): void {
  if (st.endingTimer) {
    clearTimeout(st.endingTimer);
    st.endingTimer = 0;
  }
  if (st.onEnd) {
    parts.popup.removeEventListener("transitionend", st.onEnd);
    st.onEnd = null;
  }
  parts.popup.removeAttribute("data-ending-style");
  parts.positioner.removeAttribute("data-ending-style");
}

// A popup filled after the pointer arrived would otherwise stay shut until the
// pointer left and came back, since show() runs once per hover and a DOM change
// is not a pointer event. The observer is armed only when a tooltip was asked
// to open and had nothing to say, and hide() disconnects it, so its presence
// means the pointer is still there and the tooltip is still wanted. It opens
// with no delay because the wait already happened.
function watchForText(root: Element, popup: HTMLElement): void {
  if (waiting.has(root)) return;
  const observer = new MutationObserver(() => {
    if (!popup.textContent.trim()) return;
    stopWaiting(root);
    show(root);
  });
  observer.observe(popup, { characterData: true, childList: true, subtree: true });
  waiting.set(root, observer);
}

function stopWaiting(root: Element): void {
  const observer = waiting.get(root);
  if (!observer) return;
  observer.disconnect();
  waiting.delete(root);
}

function show(root: Element): void {
  const parts = partsOf(root);
  if (!parts || root.hasAttribute("data-disabled")) return;
  // No text, no tooltip, decided here and nowhere else. The parts render
  // whatever the label is, so this is the only thing standing between an empty
  // popup and the screen -- and an empty popup still paints its box, which
  // reads as a grey pill chasing the pointer. A tooltip is a short text label,
  // so text is the whole test: an htmx swap that leaves an empty element behind
  // leaves a popup painting its box around nothing.
  //
  // Deciding it here rather than while rendering is what lets a label arrive
  // late. A popup carries its own attributes, so an expression can write into
  // one after the page is running, and emptiness is a state to wait out rather
  // than a verdict -- which is watchForText's job.
  if (!parts.popup.textContent.trim()) {
    watchForText(root, parts.popup);
    return;
  }
  stopWaiting(root);
  const st = stateOf(root);
  if (st.openTimer) {
    clearTimeout(st.openTimer);
    st.openTimer = 0;
  }
  const wasOpen = st.open;
  st.parts = parts;
  cancelEnding(st, parts);
  try {
    if (!parts.positioner.matches(":popover-open")) parts.positioner.showPopover();
  } catch {
    return;
  }
  st.open = true;
  openRoots.add(root);
  parts.positioner.setAttribute("data-open", "");
  parts.positioner.removeAttribute("data-closed");
  parts.popup.setAttribute("data-open", "");
  parts.popup.removeAttribute("data-closed");
  parts.trigger.setAttribute("data-popup-open", "");
  const anchor = anchorOf(parts.trigger);
  // track never rejects: a placement that fails is reported where it ran.
  void track(anchor, parts.positioner);
  if (wasOpen) return;
  parts.positioner.setAttribute("data-starting-style", "");
  parts.popup.setAttribute("data-starting-style", "");
  void parts.popup.offsetWidth;
  cancelAnimationFrame(st.frame);
  st.frame = requestAnimationFrame(() => {
    st.frame = 0;
    parts.positioner.removeAttribute("data-starting-style");
    parts.popup.removeAttribute("data-starting-style");
  });
}

// refreshTooltip re-places an open tooltip whose label has changed in place,
// since placement is measured from the popup's size when it opens. A closed
// tooltip needs nothing: it measures again when it next opens.
export function refreshTooltip(root: Element): void {
  if (stateOf(root).open) show(root);
}

function hide(root: Element, immediate: boolean): void {
  const st = stateOf(root);
  if (st.openTimer) {
    clearTimeout(st.openTimer);
    st.openTimer = 0;
  }
  stopWaiting(root);
  if (!st.open) return;
  const parts = st.parts ?? partsOf(root);
  st.open = false;
  openRoots.delete(root);
  lastClosedAt = Date.now();
  if (!parts) return;
  untrack(parts.positioner);
  parts.trigger.removeAttribute("data-popup-open");
  parts.positioner.removeAttribute("data-open");
  parts.positioner.setAttribute("data-closed", "");
  parts.popup.removeAttribute("data-open");
  parts.popup.setAttribute("data-closed", "");

  function done(): void {
    if (!parts) return;
    cancelEnding(st, parts);
    // The positioner can leave the DOM while it closes.
    const positioner = parts.positioner;
    whileAttached("tooltip.hide", () => {
      if (positioner.matches(":popover-open")) positioner.hidePopover();
    });
  }

  if (immediate) {
    done();
    return;
  }
  parts.positioner.setAttribute("data-ending-style", "");
  parts.popup.setAttribute("data-ending-style", "");
  st.onEnd = (event: TransitionEvent) => {
    if (event.target === parts.popup) done();
  };
  parts.popup.addEventListener("transitionend", st.onEnd);
  st.endingTimer = window.setTimeout(done, ENDING_TIMEOUT);
}

function schedule(root: Element, instant: boolean): void {
  if (root.hasAttribute("data-disabled")) return;
  const st = stateOf(root);
  if (st.open && !st.onEnd) return;
  if (st.openTimer) clearTimeout(st.openTimer);
  const delay = instant || Date.now() - lastClosedAt < GROUP_TIMEOUT ? 0 : OPEN_DELAY;
  if (delay <= 0) {
    show(root);
    return;
  }
  st.openTimer = window.setTimeout(() => {
    st.openTimer = 0;
    show(root);
  }, delay);
}

function hitOf(target: EventTarget | null): Hit | null {
  if (!(target instanceof Element)) return null;
  const trigger = target.closest<HTMLElement>(TRIGGER);
  const root = trigger?.closest(ROOT);
  if (!trigger || !root) return null;
  return { root, trigger };
}

function install(): void {
  document.addEventListener("pointerover", (event) => {
    if (event.pointerType === "touch") return;
    const hit = hitOf(event.target);
    if (!hit || !leaves(hit.trigger, event.relatedTarget)) return;
    schedule(hit.root, false);
  });

  document.addEventListener("pointerout", (event) => {
    const hit = hitOf(event.target);
    if (!hit || !leaves(hit.trigger, event.relatedTarget)) return;
    hide(hit.root, false);
  });

  document.addEventListener("focusin", (event) => {
    const hit = hitOf(event.target);
    if (!hit || !(event.target instanceof Element) || !event.target.matches(":focus-visible")) return;
    schedule(hit.root, true);
  });

  document.addEventListener("focusout", (event) => {
    const hit = hitOf(event.target);
    if (!hit || !leaves(hit.trigger, event.relatedTarget)) return;
    hide(hit.root, false);
  });

  // A press hides its trigger's tooltip, and cancels one still waiting out its
  // delay. The listener runs in the capture phase, ahead of the trigger's own
  // handlers, so one that stops propagation -- a composer's Minimize, a row's
  // inline action -- cannot leave its label floating or keep a removed root
  // counted open.
  document.addEventListener(
    "click",
    (event) => {
      const hit = hitOf(event.target);
      if (hit) hide(hit.root, true);
    },
    true,
  );

  // Every Escape hides the open tooltips, whoever claims the key: an observer
  // sees each key before any binding and never claims one.
  observe((event) => {
    if (event.key !== "Escape") return;
    openRoots.forEach((root) => {
      hide(root, true);
    });
  });

  // A tooltip's positioner closed by something other than hide -- the
  // crossing into the compact view closes every open popover (ui/dom/
  // crossing.ts) -- closes the tooltip's record with it, so it is no longer
  // marked open or placed. hide's own close finds the record already closed.
  document.addEventListener("toggle", (event) => {
    const positioner = event.target;
    if (!(positioner instanceof Element) || !positioner.matches(POSITIONER) || event.newState !== "closed") return;
    const root = positioner.closest(ROOT);
    if (root && openRoots.has(root)) hide(root, true);
  }, true);

  registerComponent(ROOT, (root, signal) => {
    signal.addEventListener("abort", () => {
      hide(root, true);
      const st = states.get(root);
      const parts = st?.parts ?? partsOf(root);
      if (st) {
        cancelAnimationFrame(st.frame);
        clearTimeout(st.endingTimer);
        if (parts) cancelEnding(st, parts);
      }
      if (parts) untrack(parts.positioner);
      openRoots.delete(root);
      states.delete(root);
    }, { once: true });
    if (root.hasAttribute("data-default-open")) show(root);
  });
  registerComponent(POSITIONER, (positioner, signal) => {
    const root = positioner.closest(ROOT);
    if (!root) return;
    const previous = states.get(root);
    if (previous?.parts && previous.parts.positioner !== positioner) {
      hide(root, true);
      cancelAnimationFrame(previous.frame);
      cancelEnding(previous, previous.parts);
      previous.parts = null;
    }
    stopWaiting(root);
    signal.addEventListener("abort", () => {
      stopWaiting(root);
      const st = states.get(root);
      if (st?.parts?.positioner !== positioner) return;
      hide(root, true);
      cancelAnimationFrame(st.frame);
      cancelEnding(st, st.parts);
      st.parts = null;
    }, { once: true });
    if (root.hasAttribute("data-default-open") && states.get(root)?.open !== true) show(root);
  });
}

if (supportsTopLayer()) install();
