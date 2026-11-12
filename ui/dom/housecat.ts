const OWN_EVENTS = "data-own-events";

function chain(event: Event): EventTarget[] {
  return event.composedPath();
}

export function eventOwner(event: Event, surface?: Element): Element | null {
  for (const node of chain(event)) {
    if (surface && node === surface) return null;
    if (node instanceof Element && node.hasAttribute(OWN_EVENTS)) {
      if (surface && !surface.contains(node)) return null;
      return node;
    }
  }
  return null;
}

// dragSelected reports whether a click selected text on the way -- the user
// dragged across the surface to select it, and the release is not a click on
// it. A component whose surface toggles on click starts its handler with
//
//   if (dragSelected(event, surface)) return;
//
// It is judged by the selection this press made, not by whether one exists
// afterwards or by how far the pointer moved. A selection left over from
// earlier must not count, or the surface goes dead until it is cleared; the
// second and third clicks of a multi-click select a word and then a line
// without any drag, and are still clicks; and a pointer that drifts a few
// pixels during a click, as it does on a trackpad or with an unsteady hand,
// is still a click.
function selectedText() {
  const selection = window.getSelection();
  return selection && !selection.isCollapsed ? selection.toString() : "";
}

let selectedAtPress = "";
document.addEventListener(
  "pointerdown",
  () => {
    selectedAtPress = selectedText();
  },
  true,
);

export function dragSelected(event: MouseEvent, surface?: Element): boolean {
  if (event.detail !== 1) return false;
  const selection = window.getSelection();
  if (!selection || selection.isCollapsed) return false;
  if (surface && !selection.containsNode(surface, true)) return false;
  return selection.toString() !== selectedAtPress;
}
