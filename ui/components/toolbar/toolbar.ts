// The delegated script every toolbar on a page shares: one key router binding
// walks any toolbar's controls and one document focusin listener keeps their
// tab stops, whatever number of toolbars the page has. Shared discovery mounts
// each toolbar, including those cloned from composer templates, and releases
// its control and resize observers when the toolbar leaves the document.
//
// What this adds is the WAI-ARIA toolbar pattern
// (https://www.w3.org/WAI/ARIA/apg/patterns/toolbar/): role="toolbar", and a
// roving tabindex that leaves the whole group a single stop in the page's tab
// sequence with the arrow keys moving between the controls inside it.

import { setAttributeIfChanged } from "../../dom/dom";
import { on } from "../../dom/keys";
import { registerComponent } from "../../dom/lifecycle";

const ROOT = "[data-toolbar]";
const MARKER = "[data-toolbar-item]";
// What can hold the roving tabindex. Kept in step with focusableTags in
// unwrapped.go, which is what fails a test when a control in a strip is not
// wrapped in an Item -- the two answer the same question, one in the browser
// and one at render time.
const FOCUSABLE = "a[href], button, input, select, textarea, summary, [tabindex]";

// APG: "Use toolbar as a grouping element only if the group contains 3 or more
// controls." A reader toolbar's actions are all conditional, so the same markup
// can render two controls on one thread and six on the next; below the floor it
// stays a plain div and every control keeps its own tab stop.
const MIN_CONTROLS = 3;

// control is the element inside an item that takes the roving tabindex: the
// first focusable one. An item is a display:contents wrapper, and what it wraps
// is usually another one or two -- a tool button inside a tooltip is a span
// inside a span before it is a button -- so the item's own firstElementChild is
// rarely the thing that can hold focus.
//
// A popover's contents sit inside its trigger's item in the DOM but are their
// own widget with their own keys, so a focusable in there is never the item's
// control.
function control(item: Element): HTMLElement | null {
  const found = Array.from(item.querySelectorAll<HTMLElement>(FOCUSABLE));
  return found.find((el) => !el.closest("[popover]")) ?? null;
}

// items are every control the toolbar owns, in document order, whatever their
// state. Membership is declared rather than guessed: a control counts because a
// toolbar.Item wraps it, not because it happens to be a <button>. Guessing
// meant every focusable thing a caller put in a strip -- a link inside a label
// chip, a close button in a popup that renders inline -- silently joined the
// arrow walk, and the fix was for the script to keep a list of what to ignore.
//
// Nothing is filtered for visibility here on purpose: a control left without a
// tabindex is a control still in the page's tab sequence, which is the thing
// this pattern exists to prevent. A compose card that is display:none until
// opened has no visible controls at all, and its toolbar still has to be
// arranged before it is shown.
function items(strip: Element): HTMLElement[] {
  return Array.from(strip.querySelectorAll<HTMLElement>(MARKER))
    .filter((item) => item.closest(ROOT) === strip)
    .map(control)
    .filter((el): el is HTMLElement => el !== null && !el.hasAttribute("disabled"));
}

// A control the user can actually reach right now. Only the arrow walk asks: a
// hidden control keeps its tabindex so that un-hiding it needs no fixup, but
// arrowing onto it would strand focus.
function reachable(el: HTMLElement): boolean {
  return el.getClientRects().length > 0;
}

function vertical(strip: Element): boolean {
  return strip.getAttribute("data-orientation") === "vertical";
}

// APG: a toolbar holding a textbox must leave the arrow keys to it. In a text
// field they are how the caret moves and Home and End are how you reach the
// ends of the value; on a select the arrows change the option. Taking them for
// the roving walk would replace all of that with focus jumping to the next
// control, with nothing to say why.
//
// A control that only looks like an input -- a checkbox, a radio, a button in
// input's clothing -- has no caret and keeps the walk.
const NO_CARET = ["button", "checkbox", "color", "image", "radio", "range", "reset", "submit"];
const TEXT_ENTRY = `input${NO_CARET.map((type) => `:not([type=${type}])`).join("")}, select, textarea`;

function typing(node: EventTarget | null): boolean {
  return node instanceof Element && node.matches(TEXT_ENTRY);
}

// demote puts a strip back the way it was rendered: no role, and every control
// its own tab stop again. The item markers stay, because the markup wrote them
// -- they say what belongs to the strip, which is true whether or not the strip
// is currently arranged as a toolbar.
function demote(el: Element): void {
  el.removeAttribute("role");
  el.removeAttribute("aria-orientation");
  items(el).forEach((item) => item.removeAttribute("tabindex"));
}

// focus moves the roving zero onto one control and takes it off the rest. APG:
// tabindex="0" belongs to "the most recently focused control", so coming back
// to the toolbar by Tab returns to where the user left it.
function focus(el: Element, next: HTMLElement): void {
  items(el).forEach((item) => {
    setAttributeIfChanged(item, "tabindex", item === next ? "0" : "-1");
  });
}

function apply(el: Element): void {
  const controls = items(el);
  if (controls.length < MIN_CONTROLS) {
    demote(el);
    return;
  }
  setAttributeIfChanged(el, "role", "toolbar");
  if (vertical(el)) setAttributeIfChanged(el, "aria-orientation", "vertical");

  // The zero has to end up on a control the user can actually reach. Keeping
  // it wherever it was is right when the holder is still reachable, and right
  // again when nothing is -- a compose card is display:none until it opens, and
  // its toolbar still has to be arranged before anyone sees it. It is wrong
  // in between: a control hidden while holding the zero is not tabbable, and it
  // is the only one offering a stop, so the whole strip leaves the tab sequence
  // rather than falling back to one stop per control.
  const walk = controls.filter(reachable);
  const held = controls.find((item) => item.getAttribute("tabindex") === "0");
  const keep = held && (!walk.length || walk.includes(held)) ? held : null;
  const first = walk[0] as HTMLElement | undefined;
  focus(el, keep ?? first ?? controls[0]);
}

function step(el: Element, from: HTMLElement, by: number): HTMLElement | null {
  const walk = items(el).filter(reachable);
  if (!walk.length) return null;
  const at = walk.indexOf(from);
  // APG makes wrapping optional and its own example wraps, which is the kinder
  // behaviour on a strip of eleven format buttons.
  return walk[at === -1 ? 0 : (at + by + walk.length) % walk.length];
}

function edge(el: Element, last: boolean): HTMLElement | null {
  const walk = items(el).filter(reachable);
  if (!walk.length) return null;
  return walk[last ? walk.length - 1 : 0];
}

function target(el: Element, from: HTMLElement, key: string): HTMLElement | null {
  const [back, forward] = vertical(el) ? ["ArrowUp", "ArrowDown"] : ["ArrowLeft", "ArrowRight"];
  switch (key) {
    case back:
      return step(el, from, -1);
    case forward:
      return step(el, from, 1);
    case "Home":
      return edge(el, false);
    case "End":
      return edge(el, true);
    default:
      return null;
  }
}

type Hit = {
  item: HTMLElement;
  strip: Element;
};

// The event's target is whatever was clicked or focused -- a glyph inside the
// button as often as the button -- so the item it sits in is what identifies
// it, and the item's control is what holds the tabindex the walk moves around.
function hitOf(node: EventTarget | null): Hit | null {
  if (!(node instanceof Element)) return null;
  const found = node.closest(ROOT);
  if (found?.getAttribute("role") !== "toolbar") return null;
  const marker = node.closest<HTMLElement>(MARKER);
  if (!marker || !found.contains(marker)) return null;
  const item = control(marker);
  return item ? { item, strip: found } : null;
}

// A toolbar rearranges itself when its own controls come and go -- the mail
// composer un-hides its templates button once the page knows there are
// templates to offer. The observer is scoped to the toolbar rather than the
// document so the cost stays with the few elements that need it, and tabindex
// is deliberately absent from the filter: apply writes it, and watching it
// would have the observer answer its own writes forever.
registerComponent(ROOT, (el, signal) => {
  const mutations = new MutationObserver(() => apply(el));
  mutations.observe(el, {
    attributeFilter: ["aria-hidden", "disabled", "hidden"],
    attributes: true,
    childList: true,
    subtree: true,
  });
  // CSS can hide controls without touching the markup -- the mail reader
  // swaps its inline actions for a More menu across the docked breakpoint --
  // and that changes the strip's size, so a resize re-arranges it too, moving
  // the zero off a control that was just hidden. apply only writes tabindex,
  // which never resizes the strip, so this cannot answer its own writes.
  const resize = typeof ResizeObserver !== "undefined" ? new ResizeObserver(() => apply(el)) : null;
  resize?.observe(el);
  signal.addEventListener("abort", () => {
    mutations.disconnect();
    resize?.disconnect();
  }, { once: true });
  apply(el);
});

function install(): void {
  // The arrow walk is a router binding on the strip. Its typing is "always"
  // because the strip decides for itself which fields keep their keys: a text
  // field does (typing), a checkbox or a radio in the strip still walks.
  on(
    ROOT,
    (event, strip) => {
      if (event.altKey || event.ctrlKey || event.metaKey || typing(event.target)) return false;
      const hit = hitOf(event.target);
      if (!hit || hit.strip !== strip) return false;
      const next = target(hit.strip, hit.item, event.key);
      if (!next || next === hit.item) return false;
      focus(hit.strip, next);
      next.focus();
      return true;
    },
    { typing: "always" },
  );

  document.addEventListener("focusin", (event) => {
    const hit = hitOf(event.target);
    if (!hit || hit.item.getAttribute("tabindex") === "0") return;
    focus(hit.strip, hit.item);
  });
}

install();
