// A drawer's two records (drawer.templ), read and written in one place. They
// are attributes on the aside DrawerUnstyled renders:
//
//   - The compact record, data-drawer-open: below docked, whether the sheet,
//     overlay or view is open.
//   - The docked record, data-docked: from docked up, whether the panel is
//     expanded, collapsed or peeking over the content.
//
// The two are independent. setOpen never touches the docked record, and
// setDocked never touches the compact one, so the sidebar's phone sheet can
// open and close while its docked panel stays as the user left it. A caller
// whose one state drives both, as the URL drives the chat rail, writes both.
//
// Nothing outside the drawer writes these attributes: callers go through
// these functions, or through window.HousecatDrawer (./drawer). This module
// has no side effects, so a script in another bundle imports it directly
// rather than waiting on ui.js for the global.

import * as Option from "effect/Option";
import * as Schema from "effect/Schema";

// SELF is a drawer that owns its compact record: ./drawer opens and closes
// it from its triggers, close buttons, backdrop and key layer.
export const SELF = "[data-slot=drawer][data-control=self]";

// SHEET is a bottom sheet: a self-controlled modal drawer that comes in from
// the bottom below docked (drawer.templ's CompactSide).
export const SHEET = `${SELF}[data-compact=modal][data-compact-side=bottom]`;

// Docked is the docked record's value (drawer.Docked).
const DockedState = Schema.Literals(["expanded", "collapsed", "peek"]);
export type Docked = typeof DockedState.Type;

// Collapse is what collapsing does to the drawer (drawer.Collapse):
// data-collapse, which the server renders from the drawer's collapsed width.
const Collapse = Schema.Literals(["none", "hidden", "column"]);

const dockedOf = Schema.decodeUnknownOption(DockedState);
const collapseOf = Schema.decodeUnknownOption(Collapse);

declare global {
  interface GlobalEventHandlersEventMap {
    // The compact record changing, from the drawer; it bubbles.
    "drawer:toggle": CustomEvent<{ open: boolean }>;
    // The docked record changing, from the drawer; it bubbles.
    "drawer:docked": CustomEvent<{ docked: Docked }>;
  }
}

// backdrop is el's scrim, the one DrawerUnstyled draws for a modal drawer.
export function backdrop(el: Element): HTMLElement | null {
  if (!el.id) return null;
  return document.querySelector<HTMLElement>(`[data-drawer-backdrop][data-drawer-for="${CSS.escape(el.id)}"]`);
}

export function isOpen(el: Element): boolean {
  return el.hasAttribute("data-drawer-open");
}

// setOpen writes the compact record and what follows it: the backdrop's
// hidden attribute and every trigger's aria-expanded. It fires drawer:toggle
// only on a change.
export function setOpen(el: Element, open: boolean): void {
  if (isOpen(el) === open) return;
  el.toggleAttribute("data-drawer-open", open);
  const back = backdrop(el);
  if (back) back.hidden = !open;
  if (el.id) {
    const triggers = document.querySelectorAll(`[data-drawer-trigger="${CSS.escape(el.id)}"]`);
    for (const trigger of triggers) trigger.setAttribute("aria-expanded", String(open));
  }
  el.dispatchEvent(new CustomEvent("drawer:toggle", { bubbles: true, detail: { open } }));
}

// docked is the docked record. The server always renders it, so a missing or
// unknown value is not a state a drawer can be in, and reads as expanded,
// which is what the drawer's classes then show.
export function docked(el: Element): Docked {
  return Option.getOrElse(dockedOf(el.getAttribute("data-docked")), (): Docked => "expanded");
}

// collapsible is whether el has a collapsed width to collapse or peek to.
export function collapsible(el: Element): boolean {
  return Option.match(collapseOf(el.getAttribute("data-collapse")), {
    onNone: () => false,
    onSome: (collapse) => collapse !== "none",
  });
}

// setDocked writes the docked record and fires drawer:docked on a change.
// A drawer with no collapsed width is always expanded, so it refuses
// collapsed and peek and returns false; otherwise it returns true.
export function setDocked(el: Element, state: Docked): boolean {
  if (state !== "expanded" && !collapsible(el)) return false;
  if (docked(el) === state) return true;
  el.setAttribute("data-docked", state);
  el.dispatchEvent(new CustomEvent("drawer:docked", { bubbles: true, detail: { docked: state } }));
  return true;
}
