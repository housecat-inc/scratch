// The delegated script every menu on a page shares. The browser opens and
// closes the popup from the trigger's popovertarget and dismisses it on an
// outside press; this module places it, opens it from the keyboard, moves the
// highlight through the items, and closes it when one is chosen -- the WAI-ARIA
// Authoring Practices menu button pattern.
//
// A menu can also open from an opener (OpenerAttributes): a button drawn
// apart from it, such as one of many rows' buttons sharing one menu. Before
// it opens, menu:before-open goes to the menu's root naming the opener, so
// the page can fill the menu in for that row; the menu is then placed
// against the opener and gives focus back to it.
//
// Every selector here is a menu-* data-slot or data-menu-opener, and
// popover.ts and selectbox.ts read only their own, so the scripts never act
// on each other's elements even though all three ride the same native
// popover.

import { closingUnheard, hide, isOpen, show, supportsTopLayer } from "../../dom/dom";
import { editable } from "../../dom/keyinfo";
import { on } from "../../dom/keys";
import { isTypeaheadKey, typeahead } from "../../dom/typeahead";
import { track, untrack } from "../../floating/floating";
import { navigating } from "../collection/collection";

const ROOT = "[data-slot=menu]";
const TRIGGER = "[data-slot=menu-trigger]";
const POSITIONER = "[data-slot=menu-positioner]";
const POPUP = "[data-slot=menu-popup]";
const ITEM = "[data-slot=menu-item]";
const OPENER = "[data-menu-opener]";

// One buffer serves every menu on the page, since only one is open at a time.
const buffer = typeahead();
const activationAnchors = new WeakMap<HTMLElement, HTMLElement>();

export function openerOf(trigger: HTMLElement): HTMLElement {
  return activationAnchors.get(trigger) ?? trigger;
}

export function activateMenu(trigger: HTMLElement, anchor: HTMLElement): void {
  activationAnchors.set(trigger, anchor);
  trigger.click();
}

// MenuBeforeOpen is the detail of menu:before-open: the opener the menu is
// about to open from.
export type MenuBeforeOpen = { opener: HTMLElement };

declare global {
  interface GlobalEventHandlersEventMap {
    "menu:before-open": CustomEvent<MenuBeforeOpen>;
  }
}

type Parts = {
  popup: HTMLElement;
  positioner: HTMLElement;
  root: HTMLElement;
  trigger: HTMLElement;
};

// Pick chooses the item a menu a key opened highlights.
type Pick = (items: HTMLElement[]) => HTMLElement | undefined;

function partsOf(el: Element): Parts | null {
  const root = el.closest<HTMLElement>(ROOT);
  if (!root) return null;
  const trigger = root.querySelector<HTMLElement>(TRIGGER);
  const positioner = root.querySelector<HTMLElement>(POSITIONER);
  const popup = positioner?.querySelector<HTMLElement>(POPUP);
  if (!trigger || !positioner || !popup) return null;
  return { popup, positioner, root, trigger };
}

// openerParts are the parts of the menu an opener names by its Root's id,
// which the positioner wears.
function openerParts(opener: HTMLElement): Parts | null {
  const positioner = document.getElementById(opener.dataset.menuOpener ?? "");
  return positioner?.matches(POSITIONER) === true ? partsOf(positioner) : null;
}

// The items the keys, typeahead and pointer can reach, in order. A disabled
// item stays in the list a reader hears but is never highlighted; a hidden
// one, or one in a hidden group, is not in the menu at all.
function itemsOf(parts: Parts): HTMLElement[] {
  return Array.from(parts.popup.querySelectorAll<HTMLElement>(ITEM)).filter(
    (item) => !item.hasAttribute("data-disabled") && !item.closest("[hidden]"),
  );
}

// The highlight is focus: focusin marks the focused item data-highlighted, so
// the pointer, the keys and typeahead all move it the same way.
function highlight(target: HTMLElement | undefined | null): void {
  target?.focus({ preventScroll: false });
}

// expanded says whether the menu is open, on its trigger and on the opener it
// opened from. Any other anchor -- the mail row a swipe tray's card opens a
// row's menu against -- is not a menu button and is left alone.
function expanded(parts: Parts, open: boolean): void {
  parts.trigger.setAttribute("aria-expanded", String(open));
  const anchor = activationAnchors.get(parts.trigger);
  if (anchor?.matches(OPENER) === true) anchor.setAttribute("aria-expanded", String(open));
}

// expand marks the trigger open and places the popup against it, once per
// opening: closed() takes data-popup-open off again. It answers when the
// popup is in place.
function expand(parts: Parts): Promise<void> {
  expanded(parts, true);
  parts.trigger.setAttribute("data-popup-open", "");
  return track(openerOf(parts.trigger), parts.positioner);
}

// A key on the trigger opens the menu and highlights the item the key asks
// for. The item is focused as soon as the popup shows, ahead of the toggle
// event, so a key pressed straight after lands in the menu. The popup only
// takes the height it scrolls in once it is placed, so the item is scrolled
// into view again then: End on a long menu would otherwise leave it below.
function openAt(parts: Parts, pick: Pick): void {
  show(parts.positioner);
  if (!isOpen(parts.positioner)) return;
  const placed = expand(parts);
  const target = pick(itemsOf(parts)) ?? parts.popup;
  highlight(target);
  void placed.then(() => target.scrollIntoView({ block: "nearest" }));
}

// A menu opened by the pointer focuses the popup with nothing highlighted, as
// Base UI's does, so the arrow keys still have somewhere to start from. One a
// key opened has already been placed and focused by openAt when its toggle
// event arrives.
function opened(parts: Parts): void {
  if (parts.trigger.hasAttribute("data-popup-open")) return;
  // expand never rejects: a placement that fails is reported where it ran.
  void expand(parts);
  const active = document.activeElement;
  if (!active || !parts.positioner.contains(active)) parts.popup.focus({ preventScroll: true });
}

// Focus left on the body or in the popup goes back to the trigger. Focus
// anywhere else stays: a press outside that landed on something focusable, or
// an item whose own click moved focus on, into a field or a dialog it opened.
function returnFocus(parts: Parts): void {
  const active = document.activeElement;
  if (!active || active === document.body || parts.positioner.contains(active)) {
    openerOf(parts.trigger).focus({ preventScroll: true });
  }
}

function closed(parts: Parts): void {
  untrack(parts.positioner);
  expanded(parts, false);
  parts.trigger.removeAttribute("data-popup-open");
  buffer.clear();
  returnFocus(parts);
  activationAnchors.delete(parts.trigger);
}

function close(parts: Parts): void {
  returnFocus(parts);
  hide(parts.positioner);
}

function labelOf(item: HTMLElement): string {
  return item.textContent;
}

// pickFor is the item a key on a menu button -- the trigger or an opener --
// opens the menu at: the first for Enter, Space and ArrowDown, the last for
// ArrowUp. The arrows stay with a list the button sits in while that list is
// moving through its rows.
function pickFor(event: KeyboardEvent, button: HTMLElement): Pick | null {
  if ((event.key === "ArrowDown" || event.key === "ArrowUp") && navigating(button)) return null;
  switch (event.key) {
    case "Enter":
    case " ":
    case "ArrowDown":
      return (items) => items[0];
    case "ArrowUp":
      return (items) => items.at(-1);
  }
  return null;
}

// openFrom opens the menu from an opener: menu:before-open first, then the
// menu against the opener, at pick's item when a key opened it and as a click
// opens it otherwise. A press on another opener while the menu is open has
// already hidden it -- by focus leaving the popup, or by light dismiss -- and
// that closing is finished here; see closingUnheard.
function openFrom(parts: Parts, opener: HTMLElement, pick?: Pick): void {
  if (closingUnheard(parts.trigger, parts.positioner)) closed(parts);
  const detail: MenuBeforeOpen = { opener };
  parts.root.dispatchEvent(new CustomEvent("menu:before-open", { bubbles: true, detail }));
  if (!pick) {
    activateMenu(parts.trigger, opener);
    return;
  }
  activationAnchors.set(parts.trigger, opener);
  openAt(parts, pick);
}

// onTriggerKey and onMenuKey answer whether they claim the key, which their
// bindings pass to the router: it prevents a claimed key and stops it, so a
// page shortcut on the same key -- j and k, or Escape closing the drawer the
// menu sits in -- never also acts. A listener ahead of the router that
// prevented the key first changes nothing. Tab acts without claiming: it
// hands focus to the trigger and leaves the key to move on from there.
function onTriggerKey(event: KeyboardEvent, parts: Parts): boolean {
  const pick = pickFor(event, parts.trigger);
  if (pick) openAt(parts, pick);
  return pick !== null;
}

function onMenuKey(event: KeyboardEvent, parts: Parts, item: HTMLElement | null): boolean {
  const items = itemsOf(parts);
  const at = item ? items.indexOf(item) : -1;
  switch (event.key) {
    case "ArrowDown":
      highlight(items[(at + 1) % items.length]);
      return true;
    case "ArrowUp":
      highlight(items[at > 0 ? at - 1 : items.length - 1]);
      return true;
    case "Home":
      highlight(items[0]);
      return true;
    case "End":
      highlight(items.at(-1));
      return true;
    case "Escape":
      close(parts);
      return true;
    // Focus goes back to the trigger first, so Tab and Shift+Tab move on from
    // the menu button rather than from wherever the hidden popup sat.
    case "Tab":
      close(parts);
      return false;
    case "Enter":
    case " ":
      // Clicking, rather than leaving the key to the browser, activates an
      // anchor on Space too, and still activates it when the page has already
      // claimed Enter for itself. A key held down from opening the menu does
      // not go on to choose the first item.
      if (item && !event.repeat) item.click();
      return true;
  }
  if (!isTypeaheadKey(event)) return false;
  const next = buffer.next(items, event.key, at >= 0 ? item : null, labelOf);
  if (next) highlight(next);
  return true;
}

function install(): void {
  document.addEventListener(
    "toggle",
    (event) => {
      const positioner = event.target;
      if (!(positioner instanceof HTMLElement) || !positioner.matches(POSITIONER)) return;
      const parts = partsOf(positioner);
      if (!parts) return;
      if (event.newState === "open") opened(parts);
      else closed(parts);
    },
    true,
  );

  // The menu's keys are router bindings on its popup and its trigger, so a key
  // the menu acts on never reaches a page shortcut on the same key -- a list
  // moving its cursor on the arrows, or opening the row under it on Enter.
  // The router also prevents the keyup of every key the menu claimed, so a
  // Space that opened the menu or chose an item never goes on to press what
  // focus moved to. The router resolves a key a script dispatched at the
  // document to the item that has focus.
  on(
    POPUP,
    (event, popup, target) => {
      const parts = partsOf(popup);
      if (!parts || !isOpen(parts.positioner)) return false;
      return onMenuKey(event, parts, target.closest<HTMLElement>(ITEM));
    },
    { typing: "never" },
  );
  on(
    TRIGGER,
    (event, trigger) => {
      const parts = partsOf(trigger);
      return !!parts && onTriggerKey(event, parts);
    },
    { typing: "never" },
  );
  on(
    OPENER,
    (event, opener) => {
      if (!(opener instanceof HTMLElement)) return false;
      const parts = openerParts(opener);
      const pick = parts ? pickFor(event, opener) : null;
      if (!parts || !pick) return false;
      openFrom(parts, opener, pick);
      return true;
    },
    { typing: "never" },
  );

  // A press on the opener the menu is open from finds it closing under it --
  // focus leaving the popup, or light dismiss -- so the click that follows
  // leaves it shut, as a second press on the trigger closes the menu. A click
  // on any other opener opens the menu from that one.
  let pressedOpen: HTMLElement | null = null;
  document.addEventListener(
    "pointerdown",
    (event) => {
      const opener = event.target instanceof Element ? event.target.closest<HTMLElement>(OPENER) : null;
      const parts = opener ? openerParts(opener) : null;
      pressedOpen = parts && isOpen(parts.positioner) && activationAnchors.get(parts.trigger) === opener
        ? opener
        : null;
    },
    true,
  );
  document.addEventListener("click", (event) => {
    const opener = event.target instanceof Element ? event.target.closest<HTMLElement>(OPENER) : null;
    const shut = opener !== null && opener === pressedOpen;
    pressedOpen = null;
    const parts = opener ? openerParts(opener) : null;
    if (!opener || !parts) return;
    event.preventDefault();
    if (!shut) openFrom(parts, opener);
  });

  // Choosing an item closes the menu once the item's own click has run, so a
  // submit item still posts and a link is still followed.
  document.addEventListener("click", (event) => {
    if (!(event.target instanceof Element)) return;
    const item = event.target.closest(ITEM);
    if (!item || item.hasAttribute("data-prevent-close")) return;
    const parts = partsOf(item);
    if (parts) close(parts);
  });

  // The pointer moves the highlight, which is focus, except away from a field
  // that sits with the menu in its positioner: a hand resting on the item that
  // showed the field would otherwise take focus back on its next move, sending
  // what is typed to typeahead and its Space to whichever item that
  // highlighted.
  document.addEventListener("pointermove", (event) => {
    if (!(event.target instanceof Element)) return;
    const item = event.target.closest<HTMLElement>(ITEM);
    if (!item || item.hasAttribute("data-disabled")) return;
    const active = document.activeElement;
    if (active === item || (editable(active) && item.closest(POSITIONER)?.contains(active) === true)) return;
    item.focus({ preventScroll: true });
  });

  // Moving the pointer off the items -- onto a label, a separator, a disabled
  // item or out of the menu -- clears the highlight, leaving focus on the
  // popup so the keys carry on from there.
  document.addEventListener("pointerout", (event) => {
    if (!(event.target instanceof Element)) return;
    const item = event.target.closest<HTMLElement>(ITEM);
    if (!item || document.activeElement !== item) return;
    const next = event.relatedTarget;
    if (next instanceof Element && next.closest(ITEM) && !next.closest("[data-disabled]")) return;
    item.closest<HTMLElement>(POPUP)?.focus({ preventScroll: true });
  });

  document.addEventListener("focusin", (event) => {
    if (event.target instanceof Element && event.target.matches(ITEM)) {
      event.target.setAttribute("data-highlighted", "");
    }
  });

  // Focus leaving the menu for anywhere but its trigger closes it. The
  // trigger is left out because pressing it moves focus there first, and its
  // own click then closes the menu.
  document.addEventListener("focusout", (event) => {
    if (!(event.target instanceof Element)) return;
    if (event.target.matches(ITEM)) event.target.removeAttribute("data-highlighted");
    const positioner = event.target.closest<HTMLElement>(POSITIONER);
    if (!positioner || !isOpen(positioner)) return;
    const next = event.relatedTarget;
    if (!(next instanceof Element) || positioner.contains(next)) return;
    const parts = partsOf(positioner);
    if (next === parts?.trigger) return;
    hide(positioner);
  });
}

if (supportsTopLayer()) install();
