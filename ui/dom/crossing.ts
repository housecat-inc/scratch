// Crossing into the compact view closes every open popover on the page:
// menus, listboxes, pickers, the popover component, tooltips, wherever they
// are. Below docked the page offers those choices differently -- a row's
// More and the sidebar's menus open as modal cards (./rowmore, ./menucard),
// and the sidebar itself folds away into the closed phone sheet -- so a
// popover opened on the desktop layout has nothing left to stand on: its
// anchor may be off screen or invisible, and it would float over a layout
// it was never placed for. It closes the way any popover closes, so each
// one's own toggle handling runs as for a light dismiss: a menu marks its
// trigger collapsed and hands focus back the way it always does, and a
// tooltip closes its own record from the same toggle
// (../components/tooltip/tooltip.ts).
//
// It is one rule, keyed on the one compact query (./viewport), rather than a
// handler per menu. Only popovers are closed. A modal dialog -- the cards
// below docked, a confirmation -- is a <dialog>, not a popover, and stays
// open, as do the drawers and the phone sheet, which are not popovers either.
// Crossing out of the compact view closes nothing: a modal card left open
// there is the card's own business.

import { hide, supportsTopLayer } from "./dom";
import { compact } from "./viewport";

// closePopovers closes every open popover. A popover nested in another
// closes with it, and hide skips one already closed.
function closePopovers(): void {
  if (!supportsTopLayer()) return;
  for (const popover of document.querySelectorAll<HTMLElement>(":popover-open")) hide(popover);
}

compact().addEventListener("change", (event) => {
  if (event.matches) closePopovers();
});
