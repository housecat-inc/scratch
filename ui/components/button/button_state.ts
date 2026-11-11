// A stateful Button or ButtonIcon (Props.States, IconProps.States) renders a
// face per state, each a button-state or button-icon-state part carrying its
// state's name and label, and hides all but the current one. setState moves it
// to another state in place: the face, data-state, the accessible name and the
// tooltip's text all change on the same element, so focus never leaves the
// button and a screen reader hears the new name. The result is exactly what
// the server renders for that state.

import { setAttributeIfChanged, setBooleanAttributeIfChanged, setTextIfChanged } from "../../dom/dom";
import { refreshTooltip } from "../tooltip/tooltip";

const FACE = ":scope > [data-slot=button-state], :scope > [data-slot=button-icon-state]";

// setState reports false, changing nothing, when the button has no state
// called name.
export function setButtonState(button: Element, name: string): boolean {
  const faces = Array.from(button.querySelectorAll<HTMLElement>(FACE));
  const next = faces.find((face) => face.dataset.name === name);
  if (!next) return false;
  let changed = false;
  for (const face of faces) {
    if (setBooleanAttributeIfChanged(face, "hidden", face !== next)) changed = true;
  }
  const label = next.dataset.label ?? "";
  if (setAttributeIfChanged(button, "data-state", name)) changed = true;
  if (setAttributeIfChanged(button, "aria-label", label)) changed = true;
  const trigger = button.closest("[data-slot=tooltip-trigger]");
  if (trigger?.matches("[data-slot=tooltip-trigger]") === true && trigger.parentElement) {
    const root = trigger.parentElement;
    const popup = root.querySelector(":scope > [data-slot=tooltip-positioner] [data-slot=tooltip-popup]");
    if (popup && setTextIfChanged(popup, label)) changed = true;
    if (changed) refreshTooltip(root);
  }
  return true;
}

declare global {
  // HousecatButton is how inline templ scripts reach setState, since they
  // cannot import from this bundle.
  interface Window {
    HousecatButton: { setState: typeof setButtonState };
  }
}

window.HousecatButton = { setState: setButtonState };
