import { toastAPI } from "./template_scripts/toast";
// A formsave form's primary button (formsave.Submit, data-form-submit) says
// how its save went: Saving… while the form posts, ✓ Saved for SAVED_MS once
// the save has redirected back to the page it was made on, and the error's
// message in the form's status, before the button, when it failed.
//
// A form drawn with formsave.Form (ui/components/formsave,
// data-form-feedback) posts through htmx's own attributes: htmx drops a
// second submit while it posts and morphs only the form back in. A Delete or
// Remove in it carries the same attributes (formsave.FormButton) and posts the
// form from the button. The hooks here (request.ts's onRequest) read the
// outcome of either request. A form whose save leads to another page is
// posted by the card instead (ContentNavigationScript, ctx.cardPost), with
// the form as the request's source, and the same hooks read its outcome.
// What is left to script is what htmx can't express: the button's ARIA
// state and accessible name, the live region's text, the ✓ that shows for a
// while, and focus.
//
// A form is found again by its id, since the answer is morphed in over it.
// The id is read as an attribute: form.id is the form's control named "id"
// when it has one.

import { setButtonState } from "../components/button/button_state";
import { busy, idle, onRequest, type RequestContext } from "./request";

const SAVED_MS = 2000;

const SUBMIT = "[data-form-submit]";
const STATUS = "[data-slot=form-status]";
const revert = new Map<string, number>();

function primary(form: Element): HTMLElement | null {
  return form.querySelector<HTMLElement>(SUBMIT);
}

function setStatus(form: Element, text: string, state: "error" | "pending" | "saved" | ""): HTMLElement | null {
  const status = form.querySelector<HTMLElement>(STATUS);
  if (!status) return null;
  if (state) status.setAttribute("data-state", state);
  else status.removeAttribute("data-state");
  status.textContent = text;
  return status;
}

// start shows the Saving… face on form's primary button, clearing a ✓ or a
// failure still showing, and returns the button, when the primary itself
// submitted the form or Enter did -- with no submitter, or through the
// section's hidden default submit. Any other button, such as a Delete, posts
// without touching it, and start returns null.
//
// The face is set here rather than from htmx's htmx-request class on the
// form: the pending state also changes the button's accessible name and its
// tooltip's text, which CSS can't, and the class can't tell the primary
// button's save from a Delete or a Preview posting from the same form.
function start(form: HTMLFormElement, by: Element | null): HTMLElement | null {
  const button = primary(form);
  if (!button) return null;
  const id = form.getAttribute("id");
  if (id) {
    clearTimeout(revert.get(id));
    revert.delete(id);
  }
  if (by && by !== button && !by.matches("[data-slot=section-default-submit]")) return null;
  // A focused button's new name isn't reliably announced, so busy says
  // Saving… in the status too, a polite live region, which Saved or the
  // failure then replaces.
  busy(button, form.querySelector(STATUS));
  return button;
}

function saved(formId: string): void {
  const form = document.getElementById(formId);
  const button = form && primary(form);
  if (!form || !button) return;
  setButtonState(button, "saved");
  setStatus(form, button.getAttribute("data-form-done") || "Saved", "saved");
  clearTimeout(revert.get(formId));
  revert.set(
    formId,
    window.setTimeout(() => {
      revert.delete(formId);
      const now = document.getElementById(formId);
      const b = now && primary(now);
      if (!now || !b) return;
      setButtonState(b, "idle");
      setStatus(now, "", "");
    }, SAVED_MS),
  );
}

// failed puts message in the form's status, or in a toast for a form with
// no status, such as a lone Run or Restore button's, or a schedule's Pause.
function failed(form: HTMLFormElement, message: string): void {
  const button = primary(form);
  if (button) setButtonState(button, "idle");
  if (!setStatus(form, message, "error")) toastAPI.show({ message });
}

// invalid moves focus to the first field the server drew invalid in form,
// once its rejection has been drawn, so a screen reader reads the field and
// the error it describes, not just the status's summary, rather than staying
// on the Save that was clicked. A field that can't take focus, such as one
// hidden behind its editor, is passed over.
function invalid(form: HTMLFormElement): void {
  for (const el of Array.from(form.querySelectorAll<HTMLElement>("[aria-invalid=true]"))) {
    el.focus();
    if (document.activeElement === el) return;
  }
}

// RETRY is what a failed post says when the answer says nothing better.
const RETRY = "That didn't work. Try again.";
const FED = "[data-form-feedback]";

// fed is the form a request posts: a formsave form, one the card posted,
// or the form of a formsave.Button button that posted it, found by id, so
// a form the answer replaced is still found; else null. Another htmx button
// in the form, such as a Preview, isn't a post of the form.
function fed(source: Element, ctx: RequestContext): HTMLFormElement | null {
  if (!(source.matches(FED) || (ctx.cardPost === true && source instanceof HTMLFormElement))) return null;
  const owner = source instanceof HTMLFormElement ? source : source.closest("form");
  const id = owner?.getAttribute("id");
  const form = id ? document.getElementById(id) : null;
  return form instanceof HTMLFormElement ? form : null;
}

// Started forms are those whose primary button shows Saving… for the
// request in flight, by id, so only a save by that button, or by Enter, is
// confirmed with ✓: a Delete or a Preview in the same form isn't.
const started = new Set<string>();

// The outcome is read in end, after the answer has been morphed in: a ✓
// written before the swap would be drawn over by the server's idle button.
// A save that redirected back to its page is confirmed and dispatches
// settings:saved on the form, whose detail's savedMs is how long its ✓
// shows. An answer that drew nothing -- echo's JSON 4xx or 5xx -- or no
// answer at all is a failure, said beside the button. A rejection drawn into
// the form takes focus to its first invalid field. Any other outcome -- a
// 200 with a warning -- is the server's, already in the form.
//
// A card post's redirect to another page is no save back to its own: the
// card swaps that page in fresh (innerHTML) rather than morphing the one that
// posted, and a form there that shares the id is not the one submitted, so
// it is neither confirmed nor reset. A formsave.Swap control outside any
// form has no status to say a failure in, so it is said in a toast.
onRequest((source, ctx) => source.matches(FED) || fed(source, ctx) !== null, {
  end(source, outcome, ctx) {
    const form = fed(source, ctx);
    const id = form?.getAttribute("id");
    if (!form || !id) {
      if (outcome.kind === "network") toastAPI.show({ message: RETRY });
      if (outcome.kind === "failed") toastAPI.show({ message: outcome.message || RETRY });
      return;
    }
    const wasStarted = started.delete(id);
    const button = primary(form);
    if (button) idle(button, form.querySelector(STATUS));
    switch (outcome.kind) {
      case "network":
        failed(form, RETRY);
        return;
      case "redirected":
        if (ctx.cardPost === true && ctx.swap !== "innerMorph") return;
        if (form.hasAttribute("data-reset-on-success")) form.reset();
        if (wasStarted) saved(id);
        form.dispatchEvent(new CustomEvent("settings:saved", { bubbles: true, detail: { savedMs: SAVED_MS } }));
        return;
      case "failed":
        failed(form, outcome.message || RETRY);
        return;
      case "rejected":
        invalid(form);
        return;
      case "aborted":
      case "swapped":
        return;
    }
  },
  start(source, ctx) {
    const form = fed(source, ctx);
    const id = form?.getAttribute("id");
    if (!form || !id) return;
    const by = source === form ? (ctx.request?.submitter ?? null) : source;
    if (start(form, by)) started.add(id);
    else started.delete(id);
  },
});
