import type { HtmxCtx } from "./content";
import { RequestAborted } from "./failures";

// HtmxWritten is the answer to a write: its status, headers and body.
export type HtmxWritten = { headers: Headers; ok: boolean; status: number; text: string };

// htmxWrite posts values to url through htmx, which applies the answer's
// out-of-band parts (the write's own swap is none) and raises the events its
// HX-Trigger names. It resolves with the answer, whatever its status, once
// htmx has swapped it, and rejects with RequestAborted when no answer came.
//
// A write the user has made has to finish even when its view goes, or the
// page does, so it is sent unlike an element's own htmx request:
// - from an element of its own, put in the document for it: htmx sends one
//   request at a time per element, holding the next back, and sends nothing
//   for an element out of the document, so a write sent beside another, or
//   after its view has gone, would wait or be dropped;
// - with keepalive when asked, for a page being left: htmx reaches fetch
//   before ajax returns, so a pagehide listener can send it;
// - with no timeout, where htmx would abort it after a minute;
// - with its failure the caller's to say, kept from request-alert's banner.
export async function htmxWrite(url: string, values: URLSearchParams, keepalive: boolean): Promise<HtmxWritten> {
  const htmx = window.htmx;
  if (!htmx) throw new RequestAborted({ cause: "htmx is not loaded" });
  const source = document.createElement("span");
  source.hidden = true;
  source.addEventListener("htmx:response:error", (event) => event.stopPropagation());
  document.body.append(source);
  const seen: { ctx?: HtmxCtx } = {};
  // htmx passes ctx.request to fetch as its init, so fields set on it here,
  // before the request is issued, are fetch's.
  const configure = (event: DocumentEventMap["htmx:config:request"]) => {
    const ctx = event.detail?.ctx;
    if (ctx?.sourceElement !== source) return;
    seen.ctx = ctx;
    Object.assign(ctx.request, { keepalive, timeout: 0 });
  };
  document.addEventListener("htmx:config:request", configure);
  let sent: Promise<unknown>;
  try {
    sent = htmx.ajax("POST", url, { source, swap: "none", values: Object.fromEntries(values) });
  } finally {
    document.removeEventListener("htmx:config:request", configure);
  }
  try {
    await sent;
  } finally {
    source.remove();
  }
  const response = seen.ctx?.response;
  if (!response) throw new RequestAborted({ cause: seen.ctx?.status });
  return { headers: response.raw.headers, ok: response.raw.ok, status: response.status, text: seen.ctx?.text ?? "" };
}
