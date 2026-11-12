// Request-lifecycle utilities for any htmx control: hooks for when a request
// starts and how it ended, and the accessible pending state a control shows
// meanwhile.
//
// htmx covers the visuals on its own -- the htmx-request class, hx-indicator,
// hx-sync to drop a second press, hx-status for what an error draws -- and a
// control should reach for those first. What it can't express is ARIA state,
// text in a live region, a confirmation that shows for a while and then
// goes, and focus, and those are what this file is for. formfeedback.ts is
// built on it, and a template can opt a lone control in with no script:
//
//   - data-request-busy on the element that issues a request -- or on the
//     submit button of a form that does -- marks that control busy while its
//     request is in flight (busy, idle). Its value, when it has one, is what
//     the live region says meanwhile, else "Saving…".
//   - data-request-status="<id>" names that live region; without it, the
//     first live region (role=status or aria-live) the control's
//     aria-describedby names is used.
//
// Inline templ scripts reach these through window.HousecatRequest.

import { setButtonState } from "../components/button/button_state";
import { classify, type Outcome, type RequestContext } from "./request_outcome";

export { classify, type Outcome, type OutcomeKind, type RequestContext } from "./request_outcome";

// Match picks the requests a hook is for, by their source element: a CSS
// selector it matches, or a predicate, which also gets the request's ctx.
// A form's submit button is not the source of the form's request -- the form
// is, with the button as ctx.request.submitter.
export type Match = string | ((source: Element, ctx: RequestContext) => boolean);

export interface RequestHooks {
  // start runs in htmx:before:request, so only for a request htmx actually
  // issues: one hx-sync drops never reaches it.
  start?: (source: Element, ctx: RequestContext) => void;
  // end runs in htmx:finally:request, after the answer has been swapped in,
  // for every request start was matched for, with how it ended.
  end?: (source: Element, outcome: Outcome, ctx: RequestContext) => void;
}

// onRequest calls hooks for each request whose source element match picks,
// and returns a function that stops it. A request is matched once, when it
// starts, so end is called for exactly the requests start was, with the same
// source element, even if the answer replaced it in the page.
export function onRequest(match: Match, hooks: RequestHooks): () => void {
  const matched = new WeakMap<RequestContext, Element>();
  const test = typeof match === "string" ? (el: Element) => el.matches(match) : match;
  const onStart = (e: DocumentEventMap["htmx:before:request"]) => {
    const ctx = e.detail?.ctx;
    const source = ctx?.sourceElement;
    if (!ctx || !source || !test(source, ctx)) return;
    matched.set(ctx, source);
    hooks.start?.(source, ctx);
  };
  const onEnd = (e: DocumentEventMap["htmx:finally:request"]) => {
    const ctx = e.detail?.ctx;
    const source = ctx && matched.get(ctx);
    if (!ctx || !source) return;
    matched.delete(ctx);
    hooks.end?.(source, classify(ctx), ctx);
  };
  document.addEventListener("htmx:before:request", onStart);
  document.addEventListener("htmx:finally:request", onEnd);
  return () => {
    document.removeEventListener("htmx:before:request", onStart);
    document.removeEventListener("htmx:finally:request", onEnd);
  };
}

const BUSY_LABEL = "Saving…";

// busy shows control's pending state: aria-disabled, never disabled, so it
// keeps focus and its place in the tab order and a screen reader calls it
// unavailable; the pending face of a stateful Button (Props.States); and, in
// liveRegion, the busy label, since a focused button's new name isn't
// reliably announced. The label is the pending face's, else control's
// data-request-busy, else "Saving…". liveRegion is visually hidden while
// pending: the control already shows it.
export function busy(control: Element, liveRegion?: Element | null): void {
  const faced = setButtonState(control, "pending");
  control.setAttribute("aria-disabled", "true");
  if (!liveRegion) return;
  const label = (faced && control.getAttribute("aria-label")) || control.getAttribute("data-request-busy")
    || BUSY_LABEL;
  liveRegion.setAttribute("data-state", "pending");
  liveRegion.textContent = label;
}

// idle reverses busy: control is no longer aria-disabled, a pending face
// returns to idle, and liveRegion is cleared if it still says the busy
// label. A face or region already moved on -- to ✓ Saved, or a failure --
// is left as it is.
export function idle(control: Element, liveRegion?: Element | null): void {
  control.removeAttribute("aria-disabled");
  if (control.getAttribute("data-state") === "pending") setButtonState(control, "idle");
  if (liveRegion?.getAttribute("data-state") === "pending") {
    liveRegion.removeAttribute("data-state");
    liveRegion.textContent = "";
  }
}

const BUSY = "[data-request-busy]";

// busyControl is the data-request-busy control a request is for: its source,
// or the button that submitted the source form.
function busyControl(source: Element, ctx: RequestContext): Element | null {
  if (source.matches(BUSY)) return source;
  const by = ctx.request?.submitter;
  return source instanceof HTMLFormElement && by?.matches(BUSY) === true ? by : null;
}

// liveRegionOf is control's live region: the one data-request-status names,
// else the first live region among those aria-describedby names.
function liveRegionOf(control: Element): Element | null {
  const named = control.getAttribute("data-request-status");
  if (named) return document.getElementById(named);
  for (const id of (control.getAttribute("aria-describedby") ?? "").split(/\s+/)) {
    const el = id ? document.getElementById(id) : null;
    if (el?.matches("[role=status], [aria-live]") === true) return el;
  }
  return null;
}

function install(): void {
  const controls = new WeakMap<RequestContext, Element>();
  onRequest((source, ctx) => busyControl(source, ctx) !== null, {
    end(_source, _outcome, ctx) {
      const control = controls.get(ctx);
      controls.delete(ctx);
      if (control) idle(control, liveRegionOf(control));
    },
    start(source, ctx) {
      const control = busyControl(source, ctx);
      if (!control) return;
      controls.set(ctx, control);
      busy(control, liveRegionOf(control));
    },
  });
}

declare global {
  // HousecatRequest is how inline templ scripts reach these utilities, since
  // they cannot import from this bundle.
  interface Window {
    HousecatRequest: { busy: typeof busy; classify: typeof classify; idle: typeof idle; onRequest: typeof onRequest };
  }
}

if (typeof document !== "undefined") {
  install();
  window.HousecatRequest = { busy, classify, idle, onRequest };
}
