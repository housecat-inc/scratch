import * as Clock from "effect/Clock";
import * as Effect from "effect/Effect";
import { cloneElement } from "../clone";
import { required } from "../required";
export type ToastOptions = {
  message: string;
  href?: string;
  actionLabel?: string;
  onAction?: (event: MouseEvent) => void;
  duration?: number;
};
type ToastNode = HTMLElement & { _hcToastStop?: (() => void) | null };
declare global {
  interface Window {
    HousecatToast: {
      viewport: () => HTMLElement;
      create: (options: ToastOptions) => HTMLElement;
      dismiss: (node: HTMLElement, duration?: number) => void;
      show: (options: ToastOptions) => HTMLElement;
    };
  }
}
const defaultToast = (() => {
  // viewport is where a toast is raised: the page's toast viewport, or
  // one made at the end of body. A viewport inside the swapped content
  // ([data-mail-content]) goes with the next swap, its toasts and their
  // Undo with it, so it is passed over.
  function viewport() {
    const nodes = document.querySelectorAll<HTMLElement>('[data-slot="toast-viewport"]');
    for (const node of nodes) {
      if (!node.closest<HTMLElement>("[data-mail-content]")) return node;
    }
    const node = clone("hc-toast-viewport-template");
    node.id = "hc-toasts";
    document.body.append(node);
    return node;
  }
  function create(options: ToastOptions) {
    const id = options.href ? "hc-toast-link-template" : "hc-toast-template";
    const node = clone(id);
    required(node.querySelector<HTMLElement>('[data-slot="toast-message"]')).textContent = options.message || "";
    const action = required(
      node.querySelector<HTMLAnchorElement | HTMLButtonElement>('[data-slot="toast-action"]'),
    );
    if (!options.actionLabel) action.remove();
    else {
      action.textContent = options.actionLabel;
      if (options.href && action instanceof HTMLAnchorElement) action.href = options.href;
      if (options.onAction) {
        action.addEventListener("click", (event) => {
          if (event instanceof MouseEvent) options.onAction?.(event);
        });
      }
    }
    return node;
  }
  // dismiss removes node after duration, replacing any countdown an
  // earlier call started; no duration holds it until the next call.
  // A mouse over it or focus in it pauses the countdown, and leaving
  // resumes it. Only a mouse pauses it: a tap on a touch screen
  // raises mouseenter with no mouseleave to follow, which would hold
  // the toast on screen for good.
  function dismiss(node: ToastNode, duration?: number) {
    if (node._hcToastStop) node._hcToastStop();
    node._hcToastStop = null;
    if (duration === undefined || duration === 0) return;
    let remaining = duration;
    let started = 0;
    let timer = 0;
    let hovered = false;
    let focused = false;
    function run() {
      if (timer || hovered || focused) return;
      started = Effect.runSync(Clock.currentTimeMillis);
      timer = window.setTimeout(() => {
        node.remove();
      }, remaining);
    }
    function pause() {
      if (!timer) return;
      clearTimeout(timer);
      timer = 0;
      remaining = Math.max(0, remaining - (Effect.runSync(Clock.currentTimeMillis) - started));
    }
    function enter(e: PointerEvent) {
      if (e.pointerType === "mouse") {
        hovered = true;
        pause();
      }
    }
    function leave(e: PointerEvent) {
      if (e.pointerType === "mouse") {
        hovered = false;
        run();
      }
    }
    function focusIn() {
      focused = true;
      pause();
    }
    function focusOut(e: FocusEvent) {
      if (e.relatedTarget instanceof Node && node.contains(e.relatedTarget)) return;
      focused = false;
      run();
    }
    node.addEventListener("pointerenter", enter);
    node.addEventListener("pointerleave", leave);
    node.addEventListener("focusin", focusIn);
    node.addEventListener("focusout", focusOut);
    node._hcToastStop = function() {
      pause();
      node.removeEventListener("pointerenter", enter);
      node.removeEventListener("pointerleave", leave);
      node.removeEventListener("focusin", focusIn);
      node.removeEventListener("focusout", focusOut);
    };
    focused = node.contains(document.activeElement);
    run();
  }
  function show(options: ToastOptions) {
    const node = create(options);
    viewport().append(node);
    dismiss(node, options.duration ?? 3000);
    return node;
  }
  return { create, dismiss, show, viewport };
})();
export const toastAPI = typeof window !== "undefined" && Reflect.has(window, "HousecatToast")
  ? window.HousecatToast
  : defaultToast;
if (typeof window !== "undefined") window.HousecatToast = toastAPI;

function clone(id: string): HTMLElement {
  const template = document.getElementById(id);
  if (!(template instanceof HTMLTemplateElement)) throw new Error(`Missing toast template ${id}`);
  const node = cloneElement(required(template.content.firstElementChild));
  if (!(node instanceof HTMLElement)) throw new Error(`Empty toast template ${id}`);
  return node;
}
