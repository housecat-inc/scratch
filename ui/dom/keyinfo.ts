// Facts about a key press that every key handler needs to agree on. Inline
// templ scripts read them from window.HousecatKeys, so a check written in a
// page script and in a component means the same thing.

import { isElement } from "./dom";

// A target that activates on Enter or Space by itself. A page-level shortcut
// that acts on Enter must leave these alone, or pressing Enter on a focused
// button runs the shortcut instead of the button.
const ACTIVATES =
  "button, a[href], summary, input[type=button], input[type=submit], input[type=reset], input[type=image], input[type=checkbox], input[type=radio], [role=button], [role=link], [role=tab], [role=menuitem], [role=menuitemcheckbox], [role=menuitemradio], [role=option], [role=checkbox], [role=radio], [role=switch]";

// A target that takes typed text or handles its own caret keys.
const EDITABLE = 'input, textarea, select, [contenteditable]:not([contenteditable="false"])';

// A widget whose role owns its own arrow navigation, on top of an editable
// field's own caret keys.
const ARROW_ROLES =
  "[role=listbox], [role=combobox], [role=menu], [role=menubar], [role=tablist], [role=toolbar], [role=radiogroup], [role=slider], [role=spinbutton], [role=grid], [role=tree], [role=treegrid], [data-slot=toolbar]";

// element is t when t is an element, in this window or a same-origin frame.
function element(t: EventTarget | null | undefined): Element | null {
  return isElement(t) ? t : null;
}

export function activates(t: EventTarget | null): boolean {
  return element(t)?.closest(ACTIVATES) != null;
}

export function editable(t: EventTarget | null): boolean {
  return element(t)?.closest(EDITABLE) != null;
}

// An input with no arrow keys of its own: a check box, a button, a file or
// colour picker. A radio group, a range and a text field do have them.
const ARROWLESS =
  "input[type=checkbox], input[type=button], input[type=submit], input[type=reset], input[type=image], input[type=file], input[type=color]";

// arrows is true when the target handles its own arrow keys: an editable
// field's caret, or a widget whose role owns arrow navigation. A page-wide
// virtual cursor yields its arrow keys only to a target like this, not to
// every focused button or link. With within, only a widget inside it counts:
// a grid cell's own listbox or toolbar keeps its arrow keys, but the grid
// around every cell does not make each of its buttons an arrow widget.
export function arrows(t: EventTarget | null, within?: Element): boolean {
  const el = element(t);
  if (editable(t) && el?.matches(ARROWLESS) !== true) return true;
  const widget = el?.closest(ARROW_ROLES);
  return widget != null && (!within || widget !== within && within.contains(widget));
}

// onPage is true when focus rests on the page itself rather than on any
// control -- the only time a page-wide virtual cursor may take arrow keys.
export function onPage(t: EventTarget | null): boolean {
  return t === document || t === document.body || t === document.documentElement;
}

// composing is true while an input method is building text. Safari reports the
// Enter that commits a composition as keyCode 229 with isComposing false, so
// both are checked.
export function composing(e: KeyboardEvent): boolean {
  // oxlint-disable-next-line typescript/no-deprecated -- keyCode 229 is the only sign of Safari's committing Enter
  return e.isComposing || e.keyCode === 229;
}

// isMac is whether the page runs on a Mac, as html[data-platform] says, read
// on every press.
export function isMac(): boolean {
  return document.documentElement.dataset.platform === "mac";
}

// mod is the platform's command modifier: Command on a Mac, Control
// elsewhere. On a Mac, Control stays with the text field, where Control-K
// deletes to the end of the line.
export function mod(e: KeyboardEvent): boolean {
  return isMac() ? e.metaKey && !e.ctrlKey : e.ctrlKey && !e.metaKey;
}

// shortcut is the modifier an app shortcut such as Command-K answers to: mod,
// and on a Mac also Control when focus is off a text field. Only a field has a
// Control-K of its own, so everywhere else Control still reaches the shortcut.
export function shortcut(e: KeyboardEvent): boolean {
  if (mod(e)) return true;
  return isMac() && e.ctrlKey && !e.metaKey && !editable(e.target);
}

declare global {
  // HousecatKeysGlobal is window.HousecatKeys: these facts, and the router in
  // ui/dom/keys.ts, which adds its own functions to the same object.
  interface HousecatKeysGlobal {
    activates: typeof activates;
    arrows: typeof arrows;
    composing: typeof composing;
    editable: typeof editable;
    mod: typeof mod;
    onPage: typeof onPage;
    shortcut: typeof shortcut;
  }
  interface Window {
    HousecatKeys: HousecatKeysGlobal;
  }
}

// KeysStub (ui/layouts/keys.templ) may have made the object already, holding
// the calls inline scripts queued before ui.js ran, so the facts join it
// rather than replace it. Window declares the object as always there, which
// it is only from here on.
const stub = window.HousecatKeys as HousecatKeysGlobal | undefined;
window.HousecatKeys = Object.assign(
  // oxlint-disable-next-line typescript/no-unsafe-type-assertion -- keys.ts adds the router's half before anything reads it
  stub ?? ({} as HousecatKeysGlobal),
  {
    activates,
    arrows,
    editable,
    mod,
    onPage,
    shortcut,
  },
);
