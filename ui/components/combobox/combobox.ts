// The delegated script every combobox on a page shares, after Base UI's
// Combobox with its input inside the popup. The browser opens and closes the
// popup from the trigger's popovertarget and dismisses it on an outside
// press; this module places it, focuses its input, filters the options as the
// input changes, moves the highlight with aria-activedescendant while focus
// stays in the input, and toggles (multiple) or picks (single) an option.
//
// A change is announced twice from the Root: combobox:before-value-change,
// cancelable, whose detail.values a listener may edit, then
// combobox:value-change once the selection that was applied is in place. The
// options' aria-selected is the selection; the hidden inputs a form submits
// and TriggerField's text follow it.
//
// Two opt-in modes, set on the Root, build a picker of tiles on the same parts.
// Grid adds Left and Right along the options, and moves Up and Down by the row
// they are drawn in, and lets Space choose while the input is empty.
// Remote leaves the options to the server: typing filters nothing here, the
// List gets a combobox:open event each time the popup opens for its own
// hx-trigger to fetch on, and once the page has replaced the List's content the
// cursor is set again.
//
// Every selector here is a combobox-* data-slot, so no other component's
// script acts on a combobox part.

import { hide, isOpen, show, supportsTopLayer } from "../../dom/dom";
import { on } from "../../dom/keys";
import { clearRequired, firstInvalid, reportRequired, validated, watchDisabled } from "../control/required";
import { anchor, first, highlight, highlighted, options, release, step, stepRow } from "../listbox/listbox";

const ROOT = "[data-slot=combobox]";
const TRIGGER = "[data-slot=combobox-trigger]";
const VALUE = "[data-slot=combobox-value]";
const POSITIONER = "[data-slot=combobox-positioner]";
const INPUT = "[data-slot=combobox-input]";
const LIST = "[data-slot=combobox-list]";
const ITEM = "[data-slot=combobox-item]";
const GROUP = "[data-slot=combobox-group]";
const EMPTY = "[data-slot=combobox-empty]";
const CREATE = "[data-slot=combobox-create]";
const CREATE_QUERY = "[data-slot=combobox-create-query]";
const FORM_VALUE = "[data-slot=combobox-form-value]";
const REQUIRED = "[data-slot=combobox-required]";

type Reason = "item-press" | "keyboard";

export type BeforeValueChange = {
  item: HTMLElement;
  reason: Reason;
  selected: boolean;
  value: string;
  values: string[];
};
export type ValueChange = { item: HTMLElement; selected: boolean; value: string; values: string[] };

type Parts = {
  create: HTMLElement | null;
  empty: HTMLElement | null;
  grid: boolean;
  input: HTMLInputElement;
  items: HTMLElement[];
  list: HTMLElement;
  multiple: boolean;
  name: string;
  positioner: HTMLElement;
  remote: boolean;
  root: HTMLElement;
  trigger: HTMLElement;
};

function partsOf(el: Element): Parts | null {
  const root = el.closest<HTMLElement>(ROOT);
  if (!root) return null;
  const trigger = root.querySelector<HTMLElement>(TRIGGER);
  const positioner = root.querySelector<HTMLElement>(POSITIONER);
  const input = root.querySelector<HTMLInputElement>(INPUT);
  const list = root.querySelector<HTMLElement>(LIST);
  if (!trigger || !positioner || !input || !list) return null;
  return {
    create: root.querySelector<HTMLElement>(CREATE),
    empty: root.querySelector<HTMLElement>(EMPTY),
    grid: root.hasAttribute("data-grid"),
    input,
    items: Array.from(list.querySelectorAll<HTMLElement>(ITEM)),
    list,
    multiple: root.hasAttribute("data-multiple"),
    name: root.getAttribute("data-name") ?? "",
    positioner,
    remote: root.hasAttribute("data-remote"),
    root,
    trigger,
  };
}

// nav is every option the cursor moves through, the create option last.
function nav(parts: Parts): HTMLElement[] {
  return parts.create ? [...parts.items, parts.create] : parts.items;
}

function labelOf(item: HTMLElement): string {
  return item.getAttribute("data-label") ?? "";
}

function valueOf(item: HTMLElement): string {
  return item.getAttribute("data-value") ?? "";
}

function current(parts: Parts): string[] {
  return parts.items.filter((item) => item.hasAttribute("data-selected")).map(valueOf);
}

// apply makes values the selection: the options, the hidden inputs, a
// required Root's validated input and TriggerField's text all follow it.
function apply(parts: Parts, values: string[]): void {
  const marked = parts.multiple ? values : values.slice(0, 1);
  parts.items.forEach((item) => {
    const on = marked.includes(valueOf(item));
    item.toggleAttribute("data-selected", on);
    item.setAttribute("aria-selected", on ? "true" : "false");
  });
  if (parts.name) {
    parts.root.querySelectorAll(FORM_VALUE).forEach((input) => input.remove());
    const held = current(parts);
    const submitted = parts.multiple ? held : [held[0] ?? ""];
    const anchorNode = parts.root.firstChild;
    submitted.forEach((value) => {
      const input = document.createElement("input");
      input.type = "hidden";
      input.name = parts.name;
      input.value = value;
      input.disabled = parts.trigger.matches(":disabled");
      input.setAttribute("data-slot", "combobox-form-value");
      parts.root.insertBefore(input, anchorNode);
    });
  }
  const required = parts.root.querySelector<HTMLTextAreaElement>(REQUIRED);
  if (required) {
    required.value = current(parts)[0] ?? "";
    if (required.validity.valid || required.disabled) clearRequired(parts.root, [parts.trigger, parts.input]);
  }
  const text = parts.trigger.querySelector<HTMLElement>(VALUE);
  if (text) {
    const chosen = parts.items.filter((item) => item.hasAttribute("data-selected")).map(labelOf);
    text.textContent = chosen.length ? chosen.join(", ") : (text.getAttribute("data-placeholder") ?? "");
  }
}

// filter shows the options whose label holds the query, offers to create the
// query when no option is called exactly that, shows Empty when no option is
// left, and moves the highlight off an option the filter hid.
function filter(parts: Parts): void {
  if (parts.remote) {
    highlight(parts.input, nav(parts), null);
    return;
  }
  const raw = parts.input.value.trim();
  const query = raw.toLowerCase();
  let exact = false;
  parts.items.forEach((item) => {
    const label = labelOf(item).toLowerCase();
    item.hidden = query !== "" && !label.includes(query);
    if (label === query) exact = true;
  });
  if (parts.create) {
    parts.create.hidden = query === "" || exact;
    const text = parts.create.querySelector(CREATE_QUERY);
    if (text) text.textContent = raw;
  }
  if (parts.empty) parts.empty.hidden = options(parts.items).length > 0;
  const at = highlighted(parts.input, nav(parts));
  if (!at || !options(nav(parts)).includes(at)) highlight(parts.input, nav(parts), first(nav(parts)));
}

// entry is where the cursor starts: the chosen option, or the first.
function entry(parts: Parts): HTMLElement | undefined {
  return options(parts.items).find((item) => item.hasAttribute("data-selected")) ?? first(nav(parts));
}

// settle sets the cursor again after the server replaced the List. htmx puts
// back the attributes of any element it replaced by id once it has settled, and
// that takes the highlight with it, so the cursor is marked again where it was,
// or on the entry when that option is gone. A cursor still marked is left alone:
// marking it again scrolls it into view, which would undo the user's scroll
// into the next page.
function settle(parts: Parts): void {
  if (!isOpen(parts.positioner)) return;
  const at = highlighted(parts.input, nav(parts));
  if (at && options(nav(parts)).includes(at)) {
    if (!at.hasAttribute("data-highlighted")) highlight(parts.input, nav(parts), at);
    return;
  }
  highlight(parts.input, nav(parts), entry(parts));
}

// anchorOf is what the popup is placed against and hands focus back to: the
// trigger, or, while the trigger has no box (hidden at this width, with
// another control opening the popup in its place), the element the Root's
// data-anchor names.
function anchorOf(parts: Parts): HTMLElement {
  if (parts.trigger.getClientRects().length > 0) return parts.trigger;
  const id = parts.root.getAttribute("data-anchor");
  return (id && document.getElementById(id)) || parts.trigger;
}

function opened(parts: Parts): void {
  parts.trigger.setAttribute("aria-expanded", "true");
  parts.trigger.setAttribute("data-popup-open", "");
  parts.input.value = "";
  filter(parts);
  if (parts.remote) {
    parts.list.scrollTop = 0;
    parts.list.dispatchEvent(new CustomEvent("combobox:open"));
  }
  // Placed once the filter has settled what the popup shows: an end-aligned
  // popup placed by its width before that would grow past its anchor's edge.
  anchor(anchorOf(parts), parts.positioner);
  highlight(parts.input, nav(parts), entry(parts));
  parts.input.focus({ preventScroll: true });
}

function closed(parts: Parts): void {
  release(parts.positioner);
  highlight(parts.input, nav(parts), null);
  parts.trigger.setAttribute("aria-expanded", "false");
  parts.trigger.removeAttribute("data-popup-open");
  const active = document.activeElement;
  if (!active || active === document.body || parts.positioner.contains(active)) {
    anchorOf(parts).focus({ preventScroll: true });
  }
}

function close(parts: Parts): void {
  hide(parts.positioner);
  anchorOf(parts).focus({ preventScroll: true });
}

// choose acts on the option under the cursor: the create option asks the
// page to create the query; any other is toggled (multiple) or picked
// (single), through the before and after events. A listener that cancels the
// before event stops the change, not the interaction (as in Base UI): a
// cancelled single pick leaves the popup open, where the user may pick
// again or dismiss it.
function choose(parts: Parts, item: HTMLElement | null | undefined, reason: Reason): void {
  if (!item) return;
  if (item === parts.create) {
    parts.root.dispatchEvent(
      new CustomEvent("combobox:create", { bubbles: true, detail: { query: parts.input.value.trim() } }),
    );
    return;
  }
  const value = valueOf(item);
  const was = current(parts);
  const selected = parts.multiple ? !was.includes(value) : true;
  const values = parts.multiple ? (selected ? [...was, value] : was.filter((v) => v !== value)) : [value];
  const detail: BeforeValueChange = { item, reason, selected, value, values };
  const before = new CustomEvent<BeforeValueChange>("combobox:before-value-change", {
    bubbles: true,
    cancelable: true,
    detail,
  });
  if (!parts.root.dispatchEvent(before)) return;
  apply(parts, detail.values);
  const applied = current(parts);
  parts.root.dispatchEvent(
    new CustomEvent<ValueChange>("combobox:value-change", {
      bubbles: true,
      detail: { item, selected: applied.includes(value), value, values: applied },
    }),
  );
  if (!parts.multiple) close(parts);
}

// across is the cursor one row down or up in a grid; from nowhere, the entry.
function across(parts: Parts, from: HTMLElement | null, down: boolean): HTMLElement | undefined {
  return from ? (stepRow(nav(parts), from, down) ?? from) : entry(parts);
}

// along is the cursor one option left or right in a grid, from nowhere the
// entry.
function along(parts: Parts, from: HTMLElement | null, delta: number): HTMLElement | undefined {
  return from ? (step(nav(parts), from, delta) ?? from) : entry(parts);
}

// onInputKey takes the keys the popup acts on and leaves everything else to
// the input, so typing filters.
function onInputKey(event: KeyboardEvent, parts: Parts): boolean {
  const at = highlighted(parts.input, nav(parts));
  const modified = event.ctrlKey || event.metaKey || event.altKey;
  switch (event.key) {
    case "ArrowDown":
      if (modified || event.shiftKey) return false;
      highlight(parts.input, nav(parts), parts.grid ? across(parts, at, true) : step(nav(parts), at, 1));
      return true;
    case "ArrowUp":
      if (modified || event.shiftKey) return false;
      highlight(parts.input, nav(parts), parts.grid ? across(parts, at, false) : step(nav(parts), at, -1));
      return true;
    case "ArrowLeft":
    case "ArrowRight":
      if (!parts.grid || modified || event.shiftKey) return false;
      highlight(parts.input, nav(parts), along(parts, at, event.key === "ArrowRight" ? 1 : -1));
      return true;
    case " ":
      if (!parts.grid || modified || parts.input.value !== "") return false;
      if (!event.repeat) choose(parts, at, "keyboard");
      return true;
    case "Enter":
      // A chord is the page's, not the popup's, so it is not claimed. But
      // the router prevents the default only for a claimed key, and Enter in
      // a text input submits the surrounding form, so it is prevented here.
      if (modified) {
        event.preventDefault();
        return false;
      }
      if (!event.repeat) choose(parts, at, "keyboard");
      return true;
    case "Escape":
      close(parts);
      return true;
    // Tab closes and stops on the trigger: the popup is in the top layer, so
    // the browser's next stop would be outside the page.
    case "Tab":
      close(parts);
      return true;
    default:
      return false;
  }
}

function onTriggerKey(event: KeyboardEvent, parts: Parts): boolean {
  if (isOpen(parts.positioner)) return false;
  if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return false;
  show(parts.positioner);
  return true;
}

function setSelected(root: Element, value: string, selected: boolean): void {
  const parts = partsOf(root.matches(ROOT) ? root : (root.querySelector(ROOT) ?? root));
  if (!parts) return;
  const was = current(parts);
  if (parts.multiple) {
    apply(parts, selected ? (was.includes(value) ? was : [...was, value]) : was.filter((v) => v !== value));
  } else {
    apply(parts, selected ? [value] : was.filter((v) => v !== value));
  }
}

// groupOf is the Group an option's data-group names, or the list itself
// when it names none the list has.
function groupOf(parts: Parts, item: HTMLElement): HTMLElement {
  const value = item.getAttribute("data-group");
  if (!value) return parts.list;
  const group = Array.from(parts.list.querySelectorAll<HTMLElement>(GROUP)).find(
    (each) => each.getAttribute("data-group") === value,
  );
  return group ?? parts.list;
}

// insertItem adds a server-rendered option in label order, within the Group
// its data-group names, under the filter as it stands, and keeps the form
// values in step with its data-selected. The option's id is made from this
// combobox's, since a server renders an Item alone and the same one may be
// inserted into several comboboxes. An option with the same value is
// replaced, not edited in place: the new one goes where its label sorts, and
// takes the highlight the old one held.
function insertItem(root: Element, item: HTMLElement): void {
  const parts = partsOf(root.matches(ROOT) ? root : (root.querySelector(ROOT) ?? root));
  if (!parts) return;
  item.id = `${parts.list.id.replace(/-list$/, "")}-item-${valueOf(item)}`;
  const same = parts.items.find((each) => valueOf(each) === valueOf(item));
  const held = !!same && isOpen(parts.positioner) && highlighted(parts.input, nav(parts)) === same;
  same?.remove();
  // The server's order: a plain code-unit comparison, among the options
  // that share the new one's parent.
  const parent = groupOf(parts, item);
  const label = labelOf(item);
  const before = parts.items.find((each) => each !== same && each.parentElement === parent && labelOf(each) > label);
  const end = parent === parts.list ? (parts.create ?? null) : null;
  parent.insertBefore(item, before ?? end);
  const next = partsOf(parts.root);
  if (!next) return;
  if (isOpen(next.positioner)) {
    filter(next);
    if (held && options(nav(next)).includes(item)) highlight(next.input, nav(next), item);
  }
  apply(next, current(next));
}

declare global {
  interface Window {
    HousecatCombobox: { insertItem: typeof insertItem; setSelected: typeof setSelected };
  }
}

function install(): void {
  window.HousecatCombobox = { insertItem, setSelected };
  watchDisabled(ROOT, TRIGGER, `${FORM_VALUE},${REQUIRED}`);

  // A submit the browser stops reports its first invalid control. When that
  // is a required combobox, the combobox reports it by opening its popup, and
  // every invalid event in the form is cancelled so the browser reports
  // nothing over it. When the first is any other control, the events are left
  // alone and the browser reports that control as usual.
  document.addEventListener(
    "invalid",
    (event) => {
      if (!validated(event.target)) return;
      const first = firstInvalid(event.target);
      if (!(first instanceof HTMLTextAreaElement) || !first.matches(REQUIRED)) return;
      const parts = partsOf(first);
      if (!parts) return;
      event.preventDefault();
      if (event.target !== first) return;
      reportRequired(parts.root, [parts.trigger, parts.input]);
      parts.trigger.focus();
      show(parts.positioner);
    },
    true,
  );

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

  // The cursor of a remote List is set once htmx has settled the results in.
  document.addEventListener("htmx:after:settle", (event) => {
    if (!(event.target instanceof Element)) return;
    const parts = partsOf(event.target);
    if (parts?.remote === true) settle(parts);
  });

  document.addEventListener("input", (event) => {
    if (!(event.target instanceof HTMLInputElement) || !event.target.matches(INPUT)) return;
    const parts = partsOf(event.target);
    if (parts) filter(parts);
  });

  // The input's keys are a router binding on the input itself, taken even
  // while typing: the binding claims only the keys the popup acts on, so a
  // printable key still types and no page shortcut sees it.
  on(
    INPUT,
    (event, input) => {
      const parts = partsOf(input);
      return !!parts && isOpen(parts.positioner) && onInputKey(event, parts);
    },
    { typing: "always" },
  );
  on(
    TRIGGER,
    (event, trigger) => {
      const parts = partsOf(trigger);
      return !!parts && onTriggerKey(event, parts);
    },
    { typing: "never" },
  );

  // Nothing in the popup (the positioner holds it) but the input takes
  // focus: a press on an option, Empty or the padding would otherwise blur
  // the input and close the popup (or strand focus on the scrolling list).
  document.addEventListener("pointerdown", (event) => {
    if (!(event.target instanceof Element) || event.target.matches(INPUT)) return;
    if (event.target.closest(POSITIONER)?.closest(ROOT)) event.preventDefault();
  });

  document.addEventListener("click", (event) => {
    if (!(event.target instanceof Element)) return;
    const item = event.target.closest<HTMLElement>(`${ITEM}, ${CREATE}`);
    if (!item) return;
    const parts = partsOf(item);
    if (parts && options(nav(parts)).includes(item)) choose(parts, item, "item-press");
  });

  // The pointer moves the cursor by moving onto an option.
  document.addEventListener("pointermove", (event) => {
    if (!(event.target instanceof Element)) return;
    const item = event.target.closest<HTMLElement>(`${ITEM}, ${CREATE}`);
    if (!item) return;
    const parts = partsOf(item);
    if (parts && options(nav(parts)).includes(item) && highlighted(parts.input, nav(parts)) !== item) {
      highlight(parts.input, nav(parts), item);
    }
  });

  // Focus leaving the input for anywhere but the popup or the trigger closes
  // the popup.
  document.addEventListener("focusout", (event) => {
    if (!(event.target instanceof Element) || !event.target.matches(INPUT)) return;
    const parts = partsOf(event.target);
    if (!parts || !isOpen(parts.positioner)) return;
    const next = event.relatedTarget;
    if (next instanceof Element && (parts.positioner.contains(next) || next === parts.trigger)) return;
    hide(parts.positioner);
  });
}

if (supportsTopLayer()) install();
