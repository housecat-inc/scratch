// Helpers every component script reaches for. They exist here rather than in
// each script because the tooltip and the popover read the same data-*
// attributes and answer the same question about a pointer event.

import { whileAttached } from "./failures";

// isElement says whether t is an element. One in a same-origin frame is an
// instance of that frame's Element, not this window's, so instanceof misses
// it; its nodeType doesn't. An inline script may pass anything.
export function isElement(t: unknown): t is Element {
  return typeof t === "object" && t !== null && "nodeType" in t && t.nodeType === 1;
}

export function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, max));
}

export function setAttributeIfChanged(el: Element, name: string, value: string | null): boolean {
  if (el.getAttribute(name) === value) return false;
  if (value === null) el.removeAttribute(name);
  else el.setAttribute(name, value);
  return true;
}

export function setBooleanAttributeIfChanged(el: Element, name: string, value: boolean): boolean {
  if (el.hasAttribute(name) === value) return false;
  el.toggleAttribute(name, value);
  return true;
}

export function setTextIfChanged(el: Node, text: string): boolean {
  if (el.textContent === text) return false;
  el.textContent = text;
  return true;
}

// intAttr reads a numeric data-* attribute, falling back when the attribute is
// absent or not a number. Every timing and offset a component takes from its
// markup comes through here.
export function intAttr(el: Element, name: string, fallback: number): number {
  const value = parseInt(el.getAttribute(name) ?? "", 10);
  return isNaN(value) ? fallback : value;
}

// leaves answers whether a pointerover or pointerout event crossed el's
// boundary rather than moving between its own descendants, which fire the same
// events. relatedTarget is null when the pointer came from outside the
// document, which counts as leaving.
export function leaves(el: Element, related: EventTarget | null): boolean {
  return !(related instanceof Node) || !el.contains(related);
}

// CONTROL matches what a user can operate, in the tab order or not: a roving
// toolbar keeps all but one of its buttons at tabindex -1, and a click on any
// of them is still a click on a control.
export const CONTROL =
  "a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]";

// supportsTopLayer gates the tooltip and popover scripts. Both are built on
// the popover API, and without it there is nothing to degrade to: the markup
// already renders the trigger, and the popup stays in the DOM unopened.
export function supportsTopLayer(): boolean {
  return typeof HTMLElement !== "undefined" && "showPopover" in HTMLElement.prototype;
}

// isOpen, show and hide drive a popover=auto or manual element for the
// scripts that open and close their own. show and hide do nothing to a popover
// taken out of the document, where the popover API throws; show answers false
// then, so a caller can drop what it queued for the opening.
export function isOpen(popover: HTMLElement): boolean {
  return popover.matches(":popover-open");
}

// closingUnheard answers whether popover has been hidden while its opener,
// which a script marks data-popup-open from the toggle event, still says it
// is open. A key or press that hides it and another that opens it again can
// both land before that event, and the browser folds the closing's toggle
// event into the opening's, so the script opening it again finishes the
// closing first or its closing handler never runs.
export function closingUnheard(opener: HTMLElement, popover: HTMLElement): boolean {
  return opener.hasAttribute("data-popup-open") && !isOpen(popover);
}

export function show(popover: HTMLElement): boolean {
  try {
    if (!isOpen(popover)) popover.showPopover();
    return true;
  } catch {
    return false;
  }
}

// hide hides popover, which may have left the document already.
export function hide(popover: HTMLElement): void {
  whileAttached("popover.hide", () => {
    if (isOpen(popover)) popover.hidePopover();
  });
}
