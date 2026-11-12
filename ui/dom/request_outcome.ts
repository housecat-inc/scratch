// How an htmx 4 request ended, read from its ctx once htmx:finally:request
// fires, after the answer has been swapped in. It is pure -- no DOM, no
// listeners -- so request.ts can wire it to the page and a unit test can
// feed it ctx-like objects in Node.

import * as Schema from "effect/Schema";

import * as Wire from "../schemas/schemas.gen";
import { readJSON } from "./failures";

// RequestContext is what this file and request.ts read of an htmx 4
// request's ctx. cardPost is ContentNavigationScript's mark on a post it
// made for a form.
export interface RequestContext {
  cardPost?: boolean;
  request?: { submitter?: Element | null };
  response?: { raw: { redirected: boolean }; status: number };
  sourceElement?: Element | null;
  status?: string;
  swap?: string;
  text?: string;
}

// OutcomeKind is how a request ended:
//
//   - redirected: a redirect was followed to an answer below 400, which was
//     swapped in -- a save that landed back on its page;
//   - swapped: an answer below 400 was drawn without a redirect, such as a
//     page with a warning;
//   - rejected: an HTML 4xx or 5xx was drawn, such as a 422 with the form's
//     field errors;
//   - failed: an answer of 400 or more was not drawn (hx-status swap:none),
//     such as echo's JSON error, whose message is kept when it has one;
//   - network: no whole answer -- none at all, or one whose body never
//     arrived because the connection dropped or htmx's timeout aborted it
//     -- and htmx says the request erred;
//   - aborted: no answer and no error, such as a request a listener
//     cancelled in htmx:before:request.
export type OutcomeKind = "aborted" | "failed" | "network" | "redirected" | "rejected" | "swapped";

export interface Outcome {
  kind: OutcomeKind;
  message?: string;
  status?: number;
}

// classify reads ctx's outcome. The htmx 4 facts it rests on: the fetch
// Response is ctx.response.raw, so raw.redirected says a redirect was
// followed; ctx.status is "swapped" once the answer is in and starts with
// "error" when the request threw; ctx.text is set once the body has been
// read, so an error without it is a body that never arrived; and ctx.swap
// is the swap htmx used, which an hx-status rule makes "none" for an answer
// it must not draw.
export function classify(ctx: RequestContext): Outcome {
  const res = ctx.response;
  const erred = ctx.status?.startsWith("error") ?? false;
  if (!res || (erred && ctx.text === undefined)) return { kind: erred ? "network" : "aborted" };
  const status = res.status;
  if (status < 400) {
    return { kind: res.raw.redirected && ctx.status === "swapped" ? "redirected" : "swapped", status };
  }
  if (ctx.swap === "none") {
    const message = messageOf(ctx.text);
    return message ? { kind: "failed", message, status } : { kind: "failed", status };
  }
  return { kind: "rejected", status };
}

// isErrorMessage is whether body is an echo JSON error body, {"message": "..."}.
export const isErrorMessage = Schema.is(Wire.ErrorMessage);

// messageOf is the message of an echo JSON error body, {"message": "..."},
// or "" for anything else, such as a proxy's HTML error page or the auth
// service's {"error": "..."}, which /api/gmail passes on. A failure's body
// need not be echo's, so one of another shape is no mismatch to report.
export function messageOf(text: string | undefined): string {
  const body = readJSON(text);
  return isErrorMessage(body) ? body.message : "";
}
