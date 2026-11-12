import * as Arr from "effect/Array";
import { required } from "./required";
import { reportError } from "./runtime";
(() => {
  const selector = "[data-sse-swap]";
  const sourceEvents = new WeakMap<Element, Set<string>>();

  const currentTargets = (source: Element, eventName: string) => {
    const targets = source instanceof HTMLElement && source.matches(selector) ? [source] : [];
    targets.push(...Array.from(source.querySelectorAll<HTMLElement>(selector)));
    return Arr.filter(
      targets,
      (target) =>
        target.dataset.sseSwap === eventName
        && target.closest(String.raw`[hx-sse\:connect]`) === source,
    );
  };

  const bind = (root: unknown) => {
    if (!(root instanceof Element || root instanceof Document || root instanceof DocumentFragment)) return;
    const targets = root instanceof HTMLElement && root.matches(selector) ? [root] : [];
    targets.push(...Array.from(root.querySelectorAll<HTMLElement>(selector)));
    for (const target of targets) {
      const source = target.closest(String.raw`[hx-sse\:connect]`);
      const eventName = target.dataset.sseSwap;
      if (!source || !eventName) continue;
      let events = sourceEvents.get(source);
      if (!events) {
        events = new Set();
        sourceEvents.set(source, events);
      }
      if (events.has(eventName)) continue;
      events.add(eventName);
      source.addEventListener(eventName, (event) => {
        for (const current of currentTargets(source, eventName)) {
          required(window.htmx)
            .swap({
              sourceElement: current,
              swap: current.getAttribute("hx-swap") || "innerHTML",
              target: current,
              text: eventData(event),
            })
            .catch((error: unknown) => reportError(error, { operation: "sse-swap" }));
        }
      });
    }
  };

  document.addEventListener("htmx:after:process", (event) => {
    if (event.target instanceof Element) bind(event.target);
  });
  document.addEventListener("htmx:after:swap", (event) => bind(event.detail?.ctx?.target ?? document));
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => bind(document));
  } else {
    bind(document);
  }
})();

function eventData(event: Event): string {
  if (!(event instanceof CustomEvent)) return "";
  const detail: unknown = event.detail;
  return detail !== null
      && typeof detail === "object"
      && "data" in detail
      && typeof detail.data === "string"
    ? detail.data
    : "";
}
