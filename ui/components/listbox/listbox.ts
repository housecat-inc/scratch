// What the kit's listbox popups share in script: the option under the cursor,
// named on the element that keeps focus with aria-activedescendant, moving
// through the options a cursor can land on, and placing the popup against its
// trigger. selectbox.ts and combobox.ts each bring their own parts and keys.

import { track, untrack } from "../../floating/floating";

// options are the items a cursor can land on: shown and enabled. A filter
// hides an option with the hidden attribute; a disabled one carries
// aria-disabled.
export function options(items: HTMLElement[]): HTMLElement[] {
  return items.filter((item) => item.hidden === false && item.getAttribute("aria-disabled") !== "true");
}

// The control keeps DOM focus the whole time the list is open and names the
// option under the cursor with aria-activedescendant, which is what the
// pattern asks of a combobox with a listbox popup. Moving real focus into the
// options would take the keys off the control and stop it being the thing
// assistive technology is reading.
export function highlight(control: HTMLElement, items: HTMLElement[], item: HTMLElement | null | undefined): void {
  items.forEach((each) => each.toggleAttribute("data-highlighted", each === item));
  if (!item) {
    control.removeAttribute("aria-activedescendant");
    return;
  }
  control.setAttribute("aria-activedescendant", item.id);
  item.scrollIntoView({ block: "nearest" });
}

export function highlighted(control: HTMLElement, items: HTMLElement[]): HTMLElement | null {
  const id = control.getAttribute("aria-activedescendant");
  return id ? (items.find((item) => item.id === id) ?? null) : null;
}

// step moves delta options on from `from`, stopping at either end. From
// nowhere, or from an option the filter has since hidden, it starts at the end
// it is heading away from.
export function step(
  items: HTMLElement[],
  from: HTMLElement | null | undefined,
  delta: number,
): HTMLElement | undefined {
  const list = options(items);
  if (list.length === 0) return undefined;
  const at = from ? list.indexOf(from) : -1;
  if (at < 0) return delta > 0 ? list[0] : list.at(-1);
  return list[Math.min(Math.max(at + delta, 0), list.length - 1)];
}

// stepRow moves to the option in the nearest row above or below `from`, in the
// column closest to it, for options a popup lays out in a grid rather than a
// column. It reads the layout, so it is for what is drawn: from the last row
// down, or the first row up, there is nowhere to go.
export function stepRow(
  items: HTMLElement[],
  from: HTMLElement,
  down: boolean,
): HTMLElement | undefined {
  const here = from.getBoundingClientRect();
  const x = here.left + here.width / 2;
  const beyond = options(items)
    .map((item) => ({ box: item.getBoundingClientRect(), item }))
    .filter(({ box }) => (down ? box.top >= here.bottom - 1 : box.bottom <= here.top + 1));
  if (beyond.length === 0) return undefined;
  const edge = (box: DOMRect) => (down ? box.top : box.bottom);
  const nearest = (down ? Math.min : Math.max)(...beyond.map(({ box }) => edge(box)));
  const apart = ({ box }: { box: DOMRect }) => Math.abs(box.left + box.width / 2 - x);
  const row = beyond.filter(({ box }) => Math.abs(edge(box) - nearest) < 1);
  return row.reduce((best, each) => (apart(each) < apart(best) ? each : best)).item;
}

export function first(items: HTMLElement[]): HTMLElement | undefined {
  return options(items)[0];
}

export function last(items: HTMLElement[]): HTMLElement | undefined {
  const list = options(items);
  return list.at(-1);
}

// anchor places an opened popup against its trigger and keeps it there; the
// popup is never narrower than the trigger, through --anchor-width.
export function anchor(trigger: HTMLElement, positioner: HTMLElement): void {
  positioner.style.setProperty("--anchor-width", trigger.offsetWidth + "px");
  // track never rejects: a placement that fails is reported where it ran.
  void track(trigger, positioner);
}

export function release(positioner: HTMLElement): void {
  untrack(positioner);
}
