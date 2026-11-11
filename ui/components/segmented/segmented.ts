// Arrow keys move between a segmented switch's items, which are tabs over
// links: one tab stop for the whole switch (the active item), Left/Right and
// Home/End to move, Enter follows the link as any link does. select() is how a
// script that changes the active item -- the mail shell's Mail | Chat switch --
// moves the thumb and the ARIA state together.

import { on } from "../../dom/keys";

const ROOT = "[data-slot=segmented]";
const ITEM = "[data-slot=segmented-item]";

function itemsOf(root: Element): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(ITEM)).filter((el) => el.closest(ROOT) === root);
}

// select(root, -1) leaves no item active, as the server draws a switch
// whose page is in none of its views: the thumb hides and the first item
// holds the tab stop.
export function select(root: Element, index: number): void {
  const items = itemsOf(root);
  if (index < -1 || index >= items.length) return;
  items.forEach((item, i) => {
    const active = i === index;
    item.setAttribute("aria-selected", String(active));
    item.tabIndex = active || (index < 0 && i === 0) ? 0 : -1;
    item.toggleAttribute("data-active", active);
  });
  root.toggleAttribute("data-none-active", index < 0);
  if (index >= 0 && root instanceof HTMLElement) root.style.setProperty("--segmented-index", String(index));
}

declare global {
  interface Window {
    HousecatSegmented: { select: typeof select };
  }
}
window.HousecatSegmented = { select };

function target(root: Element, from: HTMLElement, key: string): HTMLElement | null {
  const items = itemsOf(root);
  if (!items.length) return null;
  const at = items.indexOf(from);
  switch (key) {
    case "ArrowLeft":
      return items[at === -1 ? 0 : (at - 1 + items.length) % items.length];
    case "ArrowRight":
      return items[at === -1 ? 0 : (at + 1) % items.length];
    case "Home":
      return items[0];
    case "End":
      return items.at(-1) ?? null;
    default:
      return null;
  }
}

// The arrow walk only moves focus: activation is Enter or a click following
// the item's own href, exactly as any link does. A moved-to item also takes
// the roving tabindex so Tab away and back lands where the user left it,
// without touching aria-selected or --segmented-index -- those stay the
// server's or select()'s record of which item is active, not which one last
// held focus.
on(
  ROOT,
  (event, root) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return false;
    const from = event.target;
    if (!(from instanceof HTMLElement) || !from.matches(ITEM)) return false;
    const next = target(root, from, event.key);
    if (!next || next === from) return false;
    itemsOf(root).forEach((item) => {
      item.tabIndex = item === next ? 0 : -1;
    });
    next.focus();
    return true;
  },
  { typing: "always" },
);
