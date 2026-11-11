import * as Str from "effect/String";
declare global {
  interface DocumentEventMap {
    "htmx:response:error": CustomEvent<HtmxDetail>;
  }
}
import type { HtmxCtx, HtmxDetail } from "./content";
import { messageOf } from "./request_outcome";

const updatePoll = (ctx: HtmxCtx | null | undefined) => {
  const source = ctx?.sourceElement;
  return (
    Str.toUpperCase(ctx?.request.method ?? "") === "GET"
    && source?.id === "update-feed"
    && source.hasAttribute("data-update-poll")
  );
};
const connectionNotice = () => document.querySelector<HTMLElement>("#update-feed [data-update-connection]");
(() => {
  let hideTimer: number | undefined;
  const showAlert = (text: string) => {
    let alert = document.getElementById("chat-alert");
    if (!alert) {
      alert = document.createElement("div");
      alert.id = "chat-alert";
      alert.role = "alert";
      alert.setAttribute("aria-atomic", "true");
      alert.setAttribute("aria-live", "assertive");
      document.body.append(alert);
    }
    alert.textContent = Str.trim(text) || "request failed";
    alert.classList.add("visible");
    window.clearTimeout(hideTimer);
    hideTimer = window.setTimeout(() => alert.classList.remove("visible"), 5000);
  };

  // A gateway answers 502, 503 or 504 with a whole HTML page of its own --
  // exe.dev's proxy, while the app restarts or is briefly out of reach. The
  // app never answers so: its failures are JSON or a fragment, and its
  // fragments are never a document. Whatever an element's hx-status asks of
  // a failure, that page is not drawn anywhere, and its text is not shown.
  const gatewayStatuses = new Set([502, 503, 504]);
  const gatewayPage = (ctx: HtmxCtx | null | undefined) =>
    gatewayStatuses.has(ctx?.response?.status ?? 0)
    && /^\s*(<![dD][oO][cC][tT][yY][pP][eE]\s+[hH][tT][mM][lL]|<[hH][tT][mM][lL][\s>])/u.test(
      ctx?.text ?? "",
    );

  // responseMessage reads a failed request's body the way
  // ui/dom/formfeedback.ts does: echo's default error handler answers a 4xx
  // or 5xx with JSON {"message": …}, so that is read back out rather than
  // shown as the raw body text.
  const UNREACHABLE = "Housecat couldn't be reached. Try again in a moment.";
  const responseMessage = (ctx: HtmxCtx | null | undefined) => {
    if (gatewayPage(ctx)) return UNREACHABLE;
    return messageOf(ctx?.text) || "Request failed";
  };
  // A body that isn't JSON, such as a proxy's error page, has no message.

  // A chat reports a gateway failure in the chat rather than in a toast: a
  // message being sent says so on its own bubble, and any other request
  // there -- opening a chat, the queue, an approval, Stop -- says so in the
  // slot above the prompt, which the next send clears. It answers false
  // when the chat has no such slot, and the toast says it instead.
  const chatNotice = (ctx: HtmxCtx | null | undefined) => {
    const source = ctx?.sourceElement;
    const chat = source?.closest("[data-chat]");
    if (!chat) return false;
    if (source?.closest("[data-outgoing-state]")) return true;
    const slot = chat.querySelector("[data-chat-send-error]");
    if (!slot) return false;
    slot.textContent = UNREACHABLE;
    return true;
  };

  let reconnectTimer: number | undefined;

  const reconnect = () => {
    const notice = connectionNotice();
    if (!notice || notice.hidden === false) return;
    notice.hidden = false;
    notice.textContent = "Restarting the app… reconnecting.";
    clearTimeout(reconnectTimer);
    reconnectTimer = window.setTimeout(() => {
      const current = connectionNotice();
      if (current?.hidden === false) {
        current.textContent = "Still waiting for the app to reconnect…";
      }
    }, 30_000);
  };
  const connected = () => {
    clearTimeout(reconnectTimer);
    const notice = connectionNotice();
    if (notice) notice.hidden = true;
  };
  document.addEventListener("htmx:after:request", (e) => {
    const ctx = e.detail?.ctx;
    if (!updatePoll(ctx)) return;
    const status = ctx?.response?.status ?? 0;
    if ([502, 503, 504].includes(status)) {
      reconnect();
      e.preventDefault();
    } else {
      connected();
      if (status >= 400) {
        showAlert(responseMessage(ctx));
        e.preventDefault();
      }
    }
  });
  document.addEventListener("htmx:error", (e) => {
    const ctx = e.detail?.ctx;
    if (updatePoll(ctx) && !ctx?.response) reconnect();
  });
  // htmx merges an element's hx-status into the request after
  // htmx:after:request, so this is the last place to stop a gateway's page
  // being swapped in; the request still ends as the failure it was.
  document.addEventListener("htmx:before:swap", (e) => {
    if (gatewayPage(e.detail?.ctx)) e.preventDefault();
  });
  // A settings form reports its own failure, beside its button or in a
  // toast, so it isn't shown here again: a post by a formsave.Form form or
  // one of its FormButton buttons, both marked data-form-feedback, whose 422 is
  // drawn into the form and whose other errors formfeedback.ts writes, and
  // a card post's (ContentNavigationScript). Another htmx element inside
  // such a form, such as a Preview, is reported here as any other is.
  document.addEventListener("htmx:response:error", (e) => {
    const ctx = e.detail?.ctx;
    if (ctx?.cardPost === true || ctx?.sourceElement?.matches("[data-form-feedback]") === true) {
      return;
    }
    if (gatewayPage(ctx) && chatNotice(ctx)) return;
    showAlert(responseMessage(ctx));
  });
})();
