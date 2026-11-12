import { registerPillWorklet } from "@klinking/squircle/pill-worklet";

// Upstream observes bare utilities and this marker. Responsive utility names
// need the same marker so their shadows are measured when the shape activates.
const selector = '[class*="squircle-pill"]';
function markPills(root: ParentNode): void {
  if (root instanceof Element && root.matches(selector)) {
    root.setAttribute("data-squircle-pill", "");
  }
  for (const pill of root.querySelectorAll(selector)) {
    pill.setAttribute("data-squircle-pill", "");
  }
}
markPills(document);
new MutationObserver((records) => {
  for (const record of records) {
    if (record.type === "attributes" && record.target instanceof Element) {
      const pill = record.target;
      pill.toggleAttribute("data-squircle-pill", pill.matches(selector));
    }
    for (const node of record.addedNodes) {
      if (node instanceof Element) markPills(node);
    }
  }
}).observe(document.documentElement, {
  attributeFilter: ["class"],
  childList: true,
  subtree: true,
});

void registerPillWorklet(new URL("/assets/pill-shape.worklet.mjs", location.href)).catch(
  (err: unknown) => console.error("squircle pill worklet", err),
);
