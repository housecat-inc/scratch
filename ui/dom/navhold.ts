import { constVoid } from "effect/Function";

import { HOLD_EXEMPT, register } from "./pan";
import { buzz } from "./swipe";
import { docked, touch } from "./viewport";

// A long press on a sidebar row with a menu -- a label row, whose ⋮ opens the
// label menu -- opens that menu, as the ⋮ does: below docked the row-More
// card (ui/dom/menucard.ts), docked the popup against the ⋮. The ⋮ stays in
// sight, since nothing says a long press does anything. Android's long press
// is the contextmenu event; iOS has none, so the press is a hold on the pan
// (ui/dom/pan.ts), as a mail row's long press is. A hold takes no pan, so a
// finger that moves scrolls the sidebar or drags the sheet as before. Where
// the browser can vibrate, which Safari cannot, the press ticks as it is
// taken for a long press.

const SURFACE = "[data-slot=nav-item-surface]";
const OPENER = "[data-slot=nav-item-action] [data-menu-opener]";

// The opener a docked long press is waiting to open, until the finger lifts.
let lifting: HTMLElement | null = null;

function openerIn(target: Element): HTMLElement | null {
  return target.closest(SURFACE)?.querySelector<HTMLElement>(OPENER) ?? null;
}

// open opens the row's menu once: a press held long enough to be a hold on
// Android is its contextmenu as well. Docked the menu is its popup, a
// popover=auto, and the finger still holding the row would close it as it
// lifts: light dismiss takes a pointerup outside the popup, after a
// pointerdown made before it opened, for a press outside it. So docked the
// popup opens once that finger is up. The card below docked is a modal
// dialog, which no lift closes, and opens at once.
function open(opener: HTMLElement): void {
  if (opener.getAttribute("aria-expanded") === "true" || lifting !== null) return;
  buzz(10);
  if (!docked().matches) {
    opener.click();
    return;
  }
  lifting = opener;
  const lifted = (): void => {
    window.removeEventListener("pointerup", lifted, true);
    window.removeEventListener("pointercancel", lifted, true);
    setTimeout(() => {
      lifting = null;
      if (opener.isConnected) opener.click();
    }, 0);
  };
  window.addEventListener("pointerup", lifted, true);
  window.addEventListener("pointercancel", lifted, true);
}

register({
  cancel: constVoid,
  claim(target, event) {
    if (event.pointerType !== "touch") return null;
    const surface = target.closest<HTMLElement>(SURFACE);
    return surface?.querySelector(OPENER) ? { el: surface, kind: "hold" } : null;
  },
  end: constVoid,
  hold(surface) {
    const opener = surface.querySelector<HTMLElement>(OPENER);
    if (opener) open(opener);
  },
  move: constVoid,
  start: constVoid,
});

document.addEventListener("contextmenu", (event) => {
  if (!touch().matches) return;
  if (!(event.target instanceof Element) || event.target.closest(HOLD_EXEMPT)) return;
  const opener = openerIn(event.target);
  if (!opener) return;
  event.preventDefault();
  open(opener);
});
