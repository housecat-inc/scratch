// The delegated script every popover on a page shares. Unlike the tooltip it
// rides the browser's own popovertarget wiring: the button opens and closes the
// positioner without script, and this module reacts to the toggle event to
// place, label, focus and animate it.

import { tabbable } from "tabbable";
import { hide, intAttr, isOpen, leaves, show, supportsTopLayer } from "../../dom/dom";
import { layer } from "../../dom/keys";
import { registerComponent } from "../../dom/lifecycle";
import { track, untrack } from "../../floating/floating";

const ROOT = "[data-slot=popover]";
const POSITIONER = "[data-slot=popover-positioner]";
const POPUP = "[data-slot=popover-popup]";
const TITLE = "[data-slot=popover-title]";
const DESCRIPTION = "[data-slot=popover-description]";
// A modal popover's positioner is its Root's own child. Callers lay the parts
// out that way; one they wrap is an ordinary popover, and a non-modal popover
// nested inside a modal one is never modal itself.
const MODAL_POSITIONER = ROOT + "[data-modal] > " + POSITIONER;

const DEFAULT_DELAY = 300;
const ENDING_TIMEOUT = 300;

type State = {
  byHover: boolean;
  closeTimer: number;
  endingTimer: number;
  frame: number;
  hoverOpened: boolean;
  onEnd: ((event: TransitionEvent) => void) | null;
  openTimer: number;
};

const states = new WeakMap<Element, State>();

function stateOf(positioner: HTMLElement): State {
  let st = states.get(positioner);
  if (!st) {
    st = { byHover: false, closeTimer: 0, endingTimer: 0, frame: 0, hoverOpened: false, onEnd: null, openTimer: 0 };
    states.set(positioner, st);
  }
  return st;
}

function triggerSelector(positioner: HTMLElement): string {
  return '[popovertarget="' + CSS.escape(positioner.id) + '"]:not([popovertargetaction=hide])';
}

function triggersOf(positioner: HTMLElement): HTMLElement[] {
  if (!positioner.id) return [];
  return Array.from(document.querySelectorAll<HTMLElement>(triggerSelector(positioner)));
}

// A popover anchors to the trigger inside its own root when it has one, so a
// popover repeated down a list places against its own row rather than the
// first row's trigger.
function anchorOf(positioner: HTMLElement): HTMLElement | null {
  const root = positioner.closest(ROOT);
  const own = root && positioner.id ? root.querySelector<HTMLElement>(triggerSelector(positioner)) : null;
  if (own) return own;
  const first = triggersOf(positioner)[0] as HTMLElement | undefined;
  return first ?? null;
}

function positionerOf(trigger: Element): HTMLElement | null {
  const id = trigger.getAttribute("popovertarget");
  const positioner = id ? document.getElementById(id) : null;
  return positioner?.matches(POSITIONER) === true ? positioner : null;
}

function cancelEnding(st: State, positioner: HTMLElement, popup: HTMLElement | null): void {
  if (st.endingTimer) {
    clearTimeout(st.endingTimer);
    st.endingTimer = 0;
  }
  if (st.onEnd) {
    positioner.removeEventListener("transitionend", st.onEnd);
    st.onEnd = null;
  }
  positioner.removeAttribute("data-ending-style");
  popup?.removeAttribute("data-ending-style");
}

function label(popup: HTMLElement | null, positioner: HTMLElement): void {
  if (!popup) return;
  const title = positioner.querySelector(TITLE);
  const description = positioner.querySelector(DESCRIPTION);
  if (title?.id) popup.setAttribute("aria-labelledby", title.id);
  if (description?.id) popup.setAttribute("aria-describedby", description.id);
}

function focusInto(popup: HTMLElement | null): void {
  if (!popup) return;
  const first = tabbable(popup, { displayCheck: "full" })[0] as HTMLElement | SVGElement | undefined;
  (first ?? popup).focus({ preventScroll: true });
}

function opened(positioner: HTMLElement): void {
  const st = stateOf(positioner);
  const popup = positioner.querySelector<HTMLElement>(POPUP);
  cancelEnding(st, positioner, popup);
  positioner.setAttribute("data-open", "");
  positioner.removeAttribute("data-closed");
  if (popup) {
    popup.setAttribute("data-open", "");
    popup.removeAttribute("data-closed");
  }
  triggersOf(positioner).forEach((trigger) => {
    trigger.setAttribute("data-popup-open", "");
    trigger.setAttribute("aria-expanded", "true");
  });
  label(popup, positioner);
  const anchor = anchorOf(positioner);
  // track never rejects: a placement that fails is reported where it ran.
  if (anchor) void track(anchor, positioner);
  positioner.setAttribute("data-starting-style", "");
  popup?.setAttribute("data-starting-style", "");
  void positioner.offsetWidth;
  cancelAnimationFrame(st.frame);
  st.frame = requestAnimationFrame(() => {
    st.frame = 0;
    positioner.removeAttribute("data-starting-style");
    popup?.removeAttribute("data-starting-style");
  });
  // A popover opened by hover leaves focus where it was: moving the caret out
  // of what someone is reading or typing is worse than an unfocused popup.
  if (!st.byHover) focusInto(popup);
  st.byHover = false;
}

function closed(positioner: HTMLElement): void {
  const st = stateOf(positioner);
  const popup = positioner.querySelector<HTMLElement>(POPUP);
  st.hoverOpened = false;
  untrack(positioner);
  positioner.removeAttribute("data-open");
  positioner.setAttribute("data-closed", "");
  if (popup) {
    popup.removeAttribute("data-open");
    popup.setAttribute("data-closed", "");
  }
  triggersOf(positioner).forEach((trigger) => {
    trigger.removeAttribute("data-popup-open");
    trigger.setAttribute("aria-expanded", "false");
  });
  positioner.setAttribute("data-ending-style", "");
  popup?.setAttribute("data-ending-style", "");
  st.onEnd = (event: TransitionEvent) => {
    if (event.target === positioner) cancelEnding(st, positioner, popup);
  };
  positioner.addEventListener("transitionend", st.onEnd);
  st.endingTimer = window.setTimeout(() => {
    cancelEnding(st, positioner, popup);
  }, ENDING_TIMEOUT);
}

function hoverTriggerOf(target: EventTarget | null): HTMLElement | null {
  if (!(target instanceof Element)) return null;
  return target.closest<HTMLElement>("[popovertarget][data-open-on-hover]");
}

// stays keeps a popover open while the pointer travels between the trigger and
// the popup, which are not in the same subtree once the popup is in the top
// layer.
function stays(positioner: HTMLElement, related: EventTarget | null): boolean {
  if (!(related instanceof Element)) return false;
  if (positioner.contains(related)) return true;
  return !!related.closest(triggerSelector(positioner));
}

function scheduleHoverClose(trigger: HTMLElement, positioner: HTMLElement): void {
  const st = stateOf(positioner);
  if (st.openTimer) {
    clearTimeout(st.openTimer);
    st.openTimer = 0;
  }
  if (!st.hoverOpened) return;
  if (st.closeTimer) clearTimeout(st.closeTimer);
  st.closeTimer = window.setTimeout(() => {
    st.closeTimer = 0;
    st.hoverOpened = false;
    hide(positioner);
  }, intAttr(trigger, "data-close-delay", 0));
}

function install(): void {
  document.addEventListener(
    "toggle",
    (event) => {
      const positioner = event.target;
      if (!(positioner instanceof HTMLElement) || !positioner.matches(POSITIONER) || !positioner.isConnected) return;
      if (event.newState === "open") opened(positioner);
      else closed(positioner);
    },
    true,
  );

  document.addEventListener("focusout", (event) => {
    const from = event.target;
    if (!(from instanceof Element)) return;
    const positioner = from.closest<HTMLElement>(POSITIONER);
    if (!positioner || !isOpen(positioner)) return;
    // A modal popover is a key router layer, which brings focus back on the
    // next key, so focus leaving it does not close it; a press outside still
    // does, through the browser's light dismiss.
    if (positioner.matches(MODAL_POSITIONER)) return;
    const next = event.relatedTarget;
    if (!(next instanceof Element)) return;
    if (positioner.contains(next) || next.closest(triggerSelector(positioner))) return;
    hide(positioner);
  });

  document.addEventListener("pointerover", (event) => {
    if (event.pointerType === "touch") return;
    const trigger = hoverTriggerOf(event.target);
    if (trigger && leaves(trigger, event.relatedTarget)) {
      const positioner = positionerOf(trigger);
      if (!positioner) return;
      const st = stateOf(positioner);
      if (st.closeTimer) {
        clearTimeout(st.closeTimer);
        st.closeTimer = 0;
      }
      if (isOpen(positioner) || st.openTimer) return;
      st.openTimer = window.setTimeout(() => {
        st.openTimer = 0;
        st.byHover = true;
        st.hoverOpened = true;
        show(positioner);
      }, intAttr(trigger, "data-delay", DEFAULT_DELAY));
      return;
    }
    if (!(event.target instanceof Element)) return;
    const entered = event.target.closest<HTMLElement>(POSITIONER);
    if (!entered || !leaves(entered, event.relatedTarget)) return;
    const st = stateOf(entered);
    if (st.closeTimer) {
      clearTimeout(st.closeTimer);
      st.closeTimer = 0;
    }
  });

  document.addEventListener("pointerout", (event) => {
    const trigger = hoverTriggerOf(event.target);
    if (trigger && leaves(trigger, event.relatedTarget)) {
      const positioner = positionerOf(trigger);
      if (positioner && !stays(positioner, event.relatedTarget)) scheduleHoverClose(trigger, positioner);
      return;
    }
    if (!(event.target instanceof Element)) return;
    const left = event.target.closest<HTMLElement>(POSITIONER);
    if (!left || !leaves(left, event.relatedTarget) || stays(left, event.relatedTarget)) return;
    const hoverTrigger = anchorOf(left);
    if (hoverTrigger?.hasAttribute("data-open-on-hover") === true) scheduleHoverClose(hoverTrigger, left);
  });

  registerComponent(POSITIONER, (positioner, signal) => {
    signal.addEventListener("abort", () => {
      const st = states.get(positioner);
      if (st) {
        clearTimeout(st.openTimer);
        clearTimeout(st.closeTimer);
        cancelAnimationFrame(st.frame);
        cancelEnding(st, positioner, positioner.querySelector<HTMLElement>(POPUP));
      }
      untrack(positioner);
      states.delete(positioner);
    }, { once: true });
    if (positioner.closest(ROOT)?.hasAttribute("data-default-open") === true) show(positioner);
  });

  // A modal popover is a router layer while it is open: Tab wraps inside it,
  // a key aimed outside goes back into it, and Escape closes it.
  layer({
    escape: (_event, positioner) => {
      if (positioner instanceof HTMLElement) hide(positioner);
      return true;
    },
    id: "popover-modal",
    match: MODAL_POSITIONER + ":popover-open",
    regions: (positioner) => [positioner ?? null],
  });
}

if (supportsTopLayer()) install();
