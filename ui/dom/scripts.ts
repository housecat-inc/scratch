import * as Rec from "effect/Record";
declare global {
  interface DocumentEventMap {
    "htmx:before:settle": CustomEvent<{ newContent?: Node[] } | null>;
  }
}
import { reportError } from "./runtime";

type Script = (hook: HTMLElement, swapped: boolean) => void;
const scripts = new Map<string, Script>();
const ran = new WeakSet<HTMLElement>();
let scheduled = false;

export function registerScripts(entries: Record<string, Script>): void {
  for (const [slot, run] of Rec.toEntries(entries)) scripts.set(slot, run);
  if (scheduled) return;
  scheduled = true;
  queueMicrotask(() => {
    scheduled = false;
    runScripts(document, false);
  });
}

export function runScripts(root: Element | Document, swapped: boolean): void {
  const selector = "template[data-script]";
  const hooks = Array.from(root.querySelectorAll<HTMLElement>(selector));
  if (root instanceof HTMLElement && root.matches(selector)) hooks.unshift(root);
  for (const hook of hooks) {
    const script = scripts.get(hook.dataset.script ?? "");
    if (!script || ran.has(hook) || !hook.isConnected) continue;
    ran.add(hook);
    try {
      script(hook, swapped);
    } catch (error) {
      reportError(error, {
        operation: "template-script",
        fields: { script: hook.dataset.script ?? "" },
      });
    }
  }
}

document.addEventListener("htmx:before:settle", (event) => {
  for (const node of event.detail?.newContent ?? [event.target]) {
    if (node instanceof Element) runScripts(node, true);
  }
});
