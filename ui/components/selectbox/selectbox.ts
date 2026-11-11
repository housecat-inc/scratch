// The delegated script every selectbox on a page shares. The browser opens and
// closes the list from the trigger's popovertarget and dismisses it on an
// outside press; this module places it, moves focus through the options, and
// writes the choice into the hidden input the form submits.
//
// Every selector here is a selectbox-* data-slot, and popover.ts reads only
// popover-* ones, so the two scripts never act on each other's elements even
// though both ride the same native popover.

import { closingUnheard, hide, isOpen, show, supportsTopLayer } from "../../dom/dom";
import { on } from "../../dom/keys";
import { isTypeaheadKey, typeahead } from "../../dom/typeahead";
import { clearRequired, firstInvalid, reportRequired, validated, watchDisabled } from "../control/required";
import { anchor, first, highlight as mark, highlighted, last, release, step } from "../listbox/listbox";

const ROOT = "[data-slot=selectbox]";
const INPUT = "[data-slot=selectbox-input]";
const TRIGGER = "[data-slot=selectbox-trigger]";
const VALUE_LABEL = "[data-slot=selectbox-value] > [data-value]";
const POSITIONER = "[data-slot=selectbox-positioner]";
const ITEM = "[data-slot=selectbox-item]";
const ITEM_TEXT = "[data-slot=selectbox-item-text]";

type Parts = {
  input: HTMLInputElement | HTMLTextAreaElement;
  items: HTMLElement[];
  positioner: HTMLElement;
  root: Element;
  trigger: HTMLElement;
};

function partsOf(el: Element): Parts | null {
  const root = el.closest(ROOT);
  if (!root) return null;
  const input = root.querySelector<HTMLInputElement | HTMLTextAreaElement>(INPUT);
  const trigger = root.querySelector<HTMLElement>(TRIGGER);
  const positioner = root.querySelector<HTMLElement>(POSITIONER);
  if (!input || !trigger || !positioner) return null;
  return { input, items: Array.from(positioner.querySelectorAll<HTMLElement>(ITEM)), positioner, root, trigger };
}

function selectedItem(parts: Parts): HTMLElement | null {
  return parts.items.find((item) => item.hasAttribute("data-selected")) ?? null;
}

function textOf(item: HTMLElement): string {
  return item.querySelector(ITEM_TEXT)?.textContent ?? "";
}

// The combobox keeps focus while the list is open; see listbox.highlight.
function highlight(parts: Parts, item: HTMLElement | null | undefined): void {
  mark(parts.trigger, parts.items, item);
}

function highlightedItem(parts: Parts): HTMLElement | null {
  return highlighted(parts.trigger, parts.items);
}

// choose sets the value to item's and closes the list, focus back on the
// combobox. A list with no options has nothing to set, so it only closes.
function choose(parts: Parts, item: HTMLElement | undefined): void {
  if (item) select(parts, item);
  hide(parts.positioner);
  parts.trigger.focus({ preventScroll: true });
}

function select(parts: Parts, item: HTMLElement): void {
  const value = item.getAttribute("data-value") ?? "";
  parts.items.forEach((each) => {
    const on = each === item;
    each.toggleAttribute("data-selected", on);
    each.setAttribute("aria-selected", on ? "true" : "false");
  });
  parts.root.querySelectorAll<HTMLElement>(VALUE_LABEL).forEach((label) => {
    const on = label.getAttribute("data-value") === value;
    label.toggleAttribute("data-selected", on);
    if (on) label.removeAttribute("aria-hidden");
    else label.setAttribute("aria-hidden", "true");
  });
  if (value !== "" || parts.input.disabled) clearRequired(parts.root, [parts.trigger]);
  if (parts.input.value !== value) {
    parts.input.value = value;
    parts.input.dispatchEvent(new Event("input", { bubbles: true }));
    parts.input.dispatchEvent(new Event("change", { bubbles: true }));
  }
}

// One buffer serves every selectbox on the page, since only one list is open
// at a time.
const buffer = typeahead();

function opened(parts: Parts): void {
  parts.trigger.setAttribute("aria-expanded", "true");
  parts.trigger.setAttribute("data-popup-open", "");
  anchor(parts.trigger, parts.positioner);
  const pending = pendingCursor.get(parts.positioner);
  pendingCursor.delete(parts.positioner);
  highlight(parts, pending ?? restingItem(parts));
}

function closed(parts: Parts): void {
  // A pending cursor never outlives the open it was set for.
  pendingCursor.delete(parts.positioner);
  buffer.clear();
  release(parts.positioner);
  highlight(parts, null);
  parts.trigger.setAttribute("aria-expanded", "false");
  parts.trigger.removeAttribute("data-popup-open");
  const active = document.activeElement;
  if (!active || active === document.body || parts.positioner.contains(active)) {
    parts.trigger.focus({ preventScroll: true });
  }
}

// The keyboard cursor of a list a key opened, from that key until the toggle
// event has placed the list and highlights it. The list answers keys as soon
// as it shows, a task before that event, so a quick second key moves this
// rather than a highlight on a list not yet in place.
const pendingCursor = new WeakMap<HTMLElement, HTMLElement>();

// restingItem is where a list's cursor starts with nothing else to go on: the
// selected option, or else the first.
function restingItem(parts: Parts): HTMLElement | undefined {
  return selectedItem(parts) ?? first(parts.items);
}

// cursorOf is the option a key on the open list moves from.
function cursorOf(parts: Parts): HTMLElement | undefined {
  return pendingCursor.get(parts.positioner) ?? highlightedItem(parts) ?? restingItem(parts);
}

// moveCursor moves the keyboard cursor to item. No item -- a list with no
// option to go to -- leaves the cursor where it is.
function moveCursor(parts: Parts, item: HTMLElement | null | undefined): void {
  if (!item) return;
  if (pendingCursor.has(parts.positioner)) pendingCursor.set(parts.positioner, item);
  else highlight(parts, item);
}

// openFrom opens the list at item. A list already open keeps its cursor: no
// toggle event is coming to take up a pending one, which would otherwise
// swallow every later key. A closing not yet heard is finished first, or the
// last open's highlight would be left behind; see closingUnheard.
function openFrom(parts: Parts, item: HTMLElement | null | undefined): void {
  if (isOpen(parts.positioner)) return;
  if (closingUnheard(parts.trigger, parts.positioner)) closed(parts);
  // Set before opening, since opening can highlight in the same task, and
  // dropped again if it did not open: a list that never appeared would
  // otherwise hand a long-dead keystroke to whoever opens it next.
  if (item) pendingCursor.set(parts.positioner, item);
  if (!show(parts.positioner) && item) pendingCursor.delete(parts.positioner);
}

// onClosedKey, onOpenKey and onKey answer whether they claim the key, which
// the binding passes to the router: it prevents a claimed key and stops it. A
// listener ahead of the router that prevented the key first changes nothing.
function onClosedKey(event: KeyboardEvent, parts: Parts): boolean {
  const selected = selectedItem(parts);
  switch (event.key) {
    case "ArrowDown":
      openFrom(parts, restingItem(parts));
      return true;
    // The last option rather than the first: the combobox pattern gives Up
    // the end of the popup, which is the one row its select-only example
    // disagrees with itself about.
    case "ArrowUp":
      openFrom(parts, selected ?? last(parts.items));
      return true;
    case "Home":
      openFrom(parts, first(parts.items));
      return true;
    case "End":
      openFrom(parts, last(parts.items));
      return true;
    default:
      if (!isTypeaheadKey(event)) return false;
      openFrom(parts, buffer.next(parts.items, event.key, selected, textOf));
      return true;
  }
}

function onOpenKey(event: KeyboardEvent, parts: Parts): boolean {
  const item = cursorOf(parts);
  switch (event.key) {
    case "ArrowDown":
      moveCursor(parts, step(parts.items, item, 1));
      return true;
    case "ArrowUp":
      moveCursor(parts, step(parts.items, item, -1));
      return true;
    case "Home":
      moveCursor(parts, first(parts.items));
      return true;
    case "End":
      moveCursor(parts, last(parts.items));
      return true;
    // Space sets the value and closes, whatever is half typed: this is a
    // combobox with a listbox popup, and that pattern gives space to choosing
    // rather than to the search. A native <select size> spends it on the
    // search instead, which is a listbox and not this.
    case "Enter":
    case " ":
      choose(parts, item);
      return true;
    case "Escape":
      // The list claims Escape outright: a drawer or dialog around it that
      // also closes on Escape would otherwise shut in the same keypress.
      hide(parts.positioner);
      parts.trigger.focus({ preventScroll: true });
      return true;
    // Tab commits the option under the cursor and stops there, focus back on
    // the combobox. The pattern's own example lets the default action carry
    // focus onward, but a list in the top layer hands that onward step out of
    // the page entirely -- the address bar -- which is further than anyone
    // tabbing out of a dropdown meant to go.
    case "Tab":
      choose(parts, item);
      return true;
    default: {
      if (!isTypeaheadKey(event)) return false;
      // A query nothing answers leaves the cursor where it is. Clearing here
      // would drop the option the combobox is pointing at over a typo.
      const next = buffer.next(parts.items, event.key, item ?? null, textOf);
      if (next) moveCursor(parts, next);
      return true;
    }
  }
}

function onKey(event: KeyboardEvent, parts: Parts): boolean {
  return isOpen(parts.positioner) ? onOpenKey(event, parts) : onClosedKey(event, parts);
}

function install(): void {
  watchDisabled(ROOT, TRIGGER, INPUT);
  document.addEventListener("invalid", (event) => {
    if (!validated(event.target)) return;
    const invalid = firstInvalid(event.target);
    if (!(invalid instanceof HTMLTextAreaElement) || !invalid.matches(INPUT)) return;
    const parts = partsOf(invalid);
    if (!parts) return;
    event.preventDefault();
    if (event.target !== invalid) return;
    reportRequired(parts.root, [parts.trigger]);
    parts.trigger.focus();
    openFrom(parts, restingItem(parts));
  }, true);

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

  // The list's keys are a router binding on the combobox, so they run before
  // anything around it that also answers them -- the chat rail closes on
  // Escape at narrow widths -- and it claims exactly the keys it acted on.
  on(
    TRIGGER,
    (event, trigger) => {
      const parts = partsOf(trigger);
      return !!parts && onKey(event, parts);
    },
    { typing: "never" },
  );

  document.addEventListener("click", (event) => {
    if (!(event.target instanceof Element)) return;
    const item = event.target.closest<HTMLElement>(ITEM);
    if (!item) return;
    const parts = partsOf(item);
    if (parts) choose(parts, item);
  });

  // The pointer moves the cursor by moving, not by being somewhere: a list
  // opened from the keyboard under a resting pointer keeps the option the
  // keyboard chose until the pointer actually travels onto another one.
  document.addEventListener("pointermove", (event) => {
    if (!(event.target instanceof Element)) return;
    const item = event.target.closest<HTMLElement>(ITEM);
    if (!item) return;
    const parts = partsOf(item);
    if (parts && cursorOf(parts) !== item) moveCursor(parts, item);
  });

  // Focus stays on the combobox while the list is open, so focus leaving the
  // combobox is focus leaving the whole control.
  document.addEventListener("focusout", (event) => {
    if (!(event.target instanceof Element)) return;
    const trigger = event.target.closest<HTMLElement>(TRIGGER);
    if (!trigger) return;
    const parts = partsOf(trigger);
    if (!parts || !isOpen(parts.positioner)) return;
    const next = event.relatedTarget;
    if (next instanceof Element && (parts.positioner.contains(next) || next === parts.trigger)) return;
    hide(parts.positioner);
  });
}

if (supportsTopLayer()) install();
