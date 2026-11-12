import { dragSelected, eventOwner } from "../../dom/housecat";
import { keys as keyRouter } from "../../dom/keys";
// Forked from the templui v1.5.0 collapsible script, with two additions from
// ui/dom/housecat.ts. An event claimed by an element inside the trigger is that
// element's own -- a nested button or link, a block meant for selecting from
// -- and leaves the state alone. And a click that drag-selected text across
// the trigger is not a click, and leaves it alone too.
//
// The fork also drops upstream's tui-collapsible-open class. The root's
// data-tui-collapsible-state is the whole open state, so there is no second
// copy to keep in step and nothing to re-sync after a swap.
(function() {
  "use strict";

  function toggle(trigger: Element) {
    const root = trigger.closest('[data-tui-collapsible="root"]');
    if (!root) return;
    const open = root.getAttribute("data-tui-collapsible-state") !== "open";
    root.setAttribute("data-tui-collapsible-state", open ? "open" : "closed");
    trigger.setAttribute("aria-expanded", String(open));
  }

  function triggerFor(event: Event) {
    const trigger = event.target instanceof Element
      ? event.target.closest('[data-tui-collapsible="trigger"]')
      : null;
    if (!trigger || eventOwner(event, trigger)) return null;
    return trigger;
  }

  document.addEventListener("click", (event) => {
    const trigger = triggerFor(event);
    if (!trigger || dragSelected(event, trigger)) return;
    event.preventDefault();
    toggle(trigger);
  });

  // Enter and Space on a trigger toggle it, as a click does. The key router
  // runs the binding from the trigger, after anything inside it, so a control
  // there that owns its keys keeps them.
  keyRouter.on(
    '[data-tui-collapsible="trigger"]',
    (event, trigger) => {
      if (event.key !== " " && event.key !== "Enter") return false;
      if (eventOwner(event, trigger)) return false;
      toggle(trigger);
      return true;
    },
    { id: "tui-collapsible", typing: "never" },
  );
})();
