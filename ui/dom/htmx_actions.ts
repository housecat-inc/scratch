// What a request's element does around its htmx request, declared with data
// attributes instead of hx-on: script, so it runs without eval. Each listener
// reads the attribute from the event's target or the element around it, as
// an hx-on handler on that element hears its own and its children's events.
import { composing } from "./keyinfo";
import { registerComponent } from "./lifecycle";

function from(event: Event, selector: string): HTMLElement | null {
  return event.target instanceof Element ? event.target.closest<HTMLElement>(selector) : null;
}

function fieldErrors(text: string | undefined): string {
  try {
    const body: unknown = JSON.parse(text ?? "");
    if (typeof body !== "object" || body === null) return "";
    return Object.values(body).filter((value) => typeof value === "string").join("\n");
  } catch {
    return "";
  }
}

document.addEventListener("htmx:after:request", (event) => {
  const ctx = event.detail?.ctx;
  const status = ctx?.response?.status ?? 0;
  const ok = status > 0 && status < 400;
  const success = from(event, "[data-after-success]");
  if (success && ok) {
    const { afterSuccess, afterSuccessPath, afterSuccessTarget, afterSuccessUrl } = success.dataset;
    if (afterSuccess === "reload") window.HousecatContent.reload();
    if (afterSuccess === "close-dialog") success.closest("dialog")?.close();
    if (afterSuccess === "visit" && afterSuccessPath) window.HousecatContent.visit(afterSuccessPath);
    if (afterSuccess === "refresh" && afterSuccessUrl) {
      void window.htmx?.ajax("GET", afterSuccessUrl, { target: afterSuccessTarget, swap: "outerHTML" });
    }
  }
  if (from(event, "[data-alert-on-failure]") && !ok) {
    window.HousecatToast.show({ message: fieldErrors(ctx?.text) || "Save failed" });
  }
});

document.addEventListener("htmx:response:error", (event) => {
  const message = from(event, "[data-error-toast]")?.dataset.errorToast;
  if (message) window.HousecatToast.show({ message });
  const sendError = from(event, "[data-chat-composer-stop]")?.closest("[data-chat-composer]")?.querySelector(
    "[data-chat-send-error]",
  );
  if (sendError) sendError.textContent = "Unable to stop. Please try again.";
});

document.addEventListener("htmx:before:request", (event) => {
  const root = from(event, "[data-open-collapsible]")?.closest("[data-tui-collapsible=root]");
  if (root) {
    root.setAttribute("data-tui-collapsible-state", "open");
    root.querySelector("[data-tui-collapsible=trigger]")?.setAttribute("aria-expanded", "true");
  }
  const stop = from(event, "[data-chat-composer-stop]");
  if (stop && !stop.getAttribute("hx-post")) event.preventDefault();
});

document.addEventListener("htmx:finally:request", (event) => {
  const submit = from(event, "[data-chat-delete-submit]");
  if (!submit) return;
  setTimeout(() => {
    if (submit.isConnected && !submit.closest("[hidden]")) submit.focus();
  });
});

document.addEventListener("htmx:before:swap", (event) => {
  const keep = from(event, "[data-preserve-selection]");
  const selection = window.getSelection();
  if (keep && selection && !selection.isCollapsed && keep.contains(selection.anchorNode)) event.preventDefault();
});

document.addEventListener("click", (event) => {
  from(event, "[data-chat-composer-stop]")
    ?.closest("[data-chat-composer]")
    ?.querySelector<HTMLElement>("[data-slot=chat-composer-editor], [data-prompt-field]")
    ?.focus({ preventScroll: true });

  const trigger = from(event, "[data-chat-delete]");
  const confirmID = trigger?.getAttribute("aria-controls");
  const confirm = confirmID ? document.getElementById(confirmID) : null;
  if (confirm) {
    const error = confirm.querySelector("[data-chat-delete-error]");
    if (error) error.textContent = "";
    confirm.hidden = false;
    confirm.querySelector<HTMLElement>("[data-chat-delete-cancel]")?.focus();
  }

  const box = from(event, "[data-chat-delete-cancel]")?.closest<HTMLElement>("[data-chat-delete-confirm]");
  if (box) {
    box.hidden = true;
    const error = box.querySelector("[data-chat-delete-error]");
    if (error) error.textContent = "";
    box.closest("[data-chat]")?.querySelector<HTMLElement>("[data-chat-delete]")?.focus();
  }
});

registerComponent("form[data-submit-on-enter]", (form, signal) => {
  if (!(form instanceof HTMLFormElement)) return;
  form.addEventListener(
    "keydown",
    (event) => {
      const field = form.querySelector<HTMLTextAreaElement>("[data-prompt-field]");
      if (!field || event.target !== field || event.key !== "Enter") return;
      if (event.shiftKey || event.ctrlKey || event.metaKey || event.altKey || composing(event)) return;
      event.preventDefault();
      const submit = form.querySelector<HTMLButtonElement>("[data-prompt-submit]");
      if (submit && !submit.disabled && !field.disabled && !field.readOnly && field.value.trim()) {
        form.requestSubmit(submit);
      }
    },
    { signal },
  );
});

registerComponent("[data-reload-on]", (root, signal) => {
  const name = root instanceof HTMLElement ? root.dataset.reloadOn : undefined;
  if (name) root.addEventListener(name, () => window.HousecatContent.reload(), { signal });
});
