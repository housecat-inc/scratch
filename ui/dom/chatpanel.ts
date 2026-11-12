import * as Effect from "effect/Effect";
import * as Fiber from "effect/Fiber";
import { constVoid } from "effect/Function";
import { isAborted, isGone, reportUnless } from "./failures";

// CONTENT_ID is the mail rail's chat panel: the section codexui.Chat draws
// with codexui.MailContentID, which is both what an open swaps and the
// element holding the chat's state.
export const CONTENT_ID = "mail-chat-rail-content";

// FIELD is the composer's prompt field inside the panel.
export const FIELD = "[data-chat-form] [data-prompt-field]";

export type Ready = (field: HTMLTextAreaElement) => boolean;

// OpenOptions.fresh asks for an empty new chat, as New Chat does, over a new
// chat whose first message is under way; any other open of "new" keeps it.
export type OpenOptions = { fresh?: boolean | undefined };

export type ChatPanel = {
  // Whether an open is still under way.
  busy: () => boolean;
  // Ends the open under way and aborts the panel request still out.
  close: () => void;
  // Brings the panel to chat id once history has moved the URL to it: an
  // open of id under way goes on, a panel already showing id ends an open
  // of any other chat, and any other panel opens id.
  follow: (id: string) => void;
  // Shows chat id, "new" for an empty chat, in the panel and calls ready
  // with its composer field once a frame until ready answers true.
  open: (id: string, ready: Ready, options?: OpenOptions) => void;
};

// ChatPanelHost is the document as the lane touches it: browserHost() is the
// page's, and a test passes its own.
export type ChatPanelHost = {
  cancelAnimationFrame: (id: number) => void;
  // Listens for htmx:after:settle and returns the listener's removal.
  onSettle: (listener: () => void) => () => void;
  panel: () => Element | null;
  // Fetches url into the panel by outerHTML and returns the request's abort.
  request: (url: string) => () => void;
  requestAnimationFrame: (callback: () => void) => number;
};

// ChatPanelRuntime is the Effect the panel uses, so a test can hand it its
// own. The default names each function rather than passing the modules whole,
// so a bundle keeps only these of Effect.
export type ChatPanelRuntime = {
  Effect: Pick<typeof Effect, "andThen" | "callback" | "gen" | "runFork" | "sync" | "timeoutOption" | "void">;
  Fiber: Pick<typeof Fiber, "interrupt">;
};

const effect: ChatPanelRuntime = {
  Effect: {
    andThen: Effect.andThen,
    callback: Effect.callback,
    gen: Effect.gen,
    runFork: Effect.runFork,
    sync: Effect.sync,
    timeoutOption: Effect.timeoutOption,
    void: Effect.void,
  },
  Fiber: { interrupt: Fiber.interrupt },
};

// Lane is the fiber of the open under way.
type Lane = Fiber.Fiber<unknown>;

// chatHref is the rail's request for chat id, the same /codex fragment the
// rail's links ask for on the mail surface.
export function chatHref(id: string): string {
  return "/codex?" + new URLSearchParams([["surface", "mail"], ["chat", id]]).toString();
}

// STARTED marks a chat that holds a message: a turn of its transcript, or a
// message the page is sending or failed to send, which a send draws as it
// starts.
export const STARTED = "[data-outgoing], section[data-turn-id]";

// mounted is whether panel already shows chat id. The panel is itself the
// [data-chat] section, and a new chat's has no thread. A new chat's thread is
// set only once its first send returns, and the panel then moves to that
// chat, so a new chat's panel holding a message is the chat that send
// starts. It shows "new" unless fresh asks for an empty chat: replacing it
// detaches the send, whose thread-created then never reaches the document,
// so only an explicit New Chat may.
export function mounted(panel: Element, id: string, fresh = false): boolean {
  if (!panel.hasAttribute("data-chat")) return false;
  if (id !== "new") return panel.getAttribute("data-selected-thread") === id;
  return panel.getAttribute("data-selected-thread") === "" && !(fresh && panel.querySelector(STARTED));
}

// unsettled is whether panel, or the composer in it, is content htmx has
// swapped in and not settled yet: htmx marks new content htmx-added until it
// has fired htmx:after:settle, and binds its hx-on:* handlers only then.
export function unsettled(panel: Element): boolean {
  return panel.classList.contains("htmx-added") || !!panel.querySelector(FIELD)?.closest(".htmx-added");
}

export function browserHost(): ChatPanelHost {
  return {
    cancelAnimationFrame: (id) => window.cancelAnimationFrame(id),
    onSettle(listener) {
      document.addEventListener("htmx:after:settle", listener);
      return () => document.removeEventListener("htmx:after:settle", listener);
    },
    panel: () => document.getElementById(CONTENT_ID),
    // htmx dispatches htmx:config:request inside ajax(), before its first
    // await, for a source without hx-vals or hx-headers, so the request's
    // own abort is in hand when ajax returns. It is used rather than
    // htmx:abort on the panel: a replaced request's late finally clears the
    // panel queue's record of the one that replaced it, which htmx:abort
    // reads.
    request(url) {
      const panel = document.getElementById(CONTENT_ID);
      let abort: () => void = constVoid;
      const capture = (e: DocumentEventMap["htmx:config:request"]) => {
        const ctx = e.detail?.ctx;
        if (ctx?.sourceElement === panel) abort = () => ctx.request.abort();
      };
      document.addEventListener("htmx:config:request", capture);
      // htmx says a request that failed in its own events, which the request
      // alert shows. A request the network cut off or the panel aborted, and
      // a panel gone before it could load, are expected; anything else is
      // reported.
      void window.htmx?.ajax("GET", url, { swap: "outerHTML", target: "#" + CONTENT_ID }).catch(
        reportUnless({ operation: "chat.panel-load" }, isAborted, isGone),
      );
      document.removeEventListener("htmx:config:request", capture);
      return () => abort();
    },
    requestAnimationFrame: (callback) => window.requestAnimationFrame(callback),
  };
}

export function createChatPanel(
  { Effect, Fiber }: ChatPanelRuntime = effect,
  host: ChatPanelHost = browserHost(),
): ChatPanel {
  // current is the lane: the fiber of the open under way, and opening the
  // chat it opens. They are cleared by identity, not by a finalizer: a
  // superseded fiber finishes its interruption after current names the newer
  // one.
  let current: Lane | null = null;
  let opening: string | null = null;
  // abort cancels the panel request the latest open made. The deadline ends
  // the wait and leaves the request to land, so the rail never stays on the
  // old chat under the new one's URL; a close or a newer open aborts it,
  // synchronously, before any fiber runs.
  const none = constVoid;
  let abort = none;

  const frame = Effect.callback<void>((resume) => {
    const id = host.requestAnimationFrame(() => resume(Effect.void));
    return Effect.sync(() => host.cancelAnimationFrame(id));
  });

  // settled resumes once a settle leaves a panel other than stale in place,
  // or any panel when stale is null. The listener removes itself as it
  // resumes; the cleanup, which runs only on interruption, removes it
  // otherwise.
  const settled = (stale: Element | null) =>
    Effect.callback<void>((resume) => {
      const off = host.onSettle(() => {
        const panel = host.panel();
        if (!panel || panel === stale) return;
        off();
        resume(Effect.void);
      });
      return Effect.sync(off);
    });

  // The frame after settled matters: htmx binds the swapped composer's
  // hx-on:input after it fires htmx:after:settle, and a draft filled before
  // that would never become the message Send reads. A panel, mounted or just
  // fetched, may be one htmx has swapped in and not settled yet -- a settle
  // elsewhere can end the wait for the fetched one before its own -- which
  // waits the same.
  const program = (id: string, ready: Ready, fresh: boolean) =>
    Effect.gen(function*() {
      const panel = host.panel();
      if (!panel) return;
      if (!mounted(panel, id, fresh)) {
        abort = host.request(chatHref(id));
        yield* settled(panel);
        abort = none;
        yield* frame;
      }
      for (let shown = host.panel(); shown && unsettled(shown); shown = host.panel()) {
        yield* settled(null);
        yield* frame;
      }
      for (;;) {
        const field = host.panel()?.querySelector<HTMLTextAreaElement>(FIELD);
        if (field && ready(field)) return;
        yield* frame;
      }
    }).pipe(Effect.timeoutOption("5 seconds"));

  function cancel(): void {
    abort();
    abort = none;
  }

  function close(): void {
    cancel();
    if (current) Effect.runFork(Fiber.interrupt(current));
  }

  function open(id: string, ready: Ready, options: OpenOptions = {}): void {
    cancel();
    const prev = current;
    const next = program(id, ready, options.fresh === true);
    const fiber = Effect.runFork(prev ? Effect.andThen(Fiber.interrupt(prev), next) : next);
    current = fiber;
    opening = id;
    fiber.addObserver(() => {
      if (current !== fiber) return;
      current = null;
      opening = null;
    });
  }

  return {
    busy: () => current !== null,
    close,
    follow(id) {
      if (current && opening === id) return;
      const panel = host.panel();
      if (panel && mounted(panel, id)) close();
      else open(id, () => true);
    },
    open,
  };
}

declare global {
  interface Window {
    HousecatChatPanel?: ChatPanel;
  }
}

if (typeof window !== "undefined") {
  window.HousecatChatPanel = createChatPanel();
}
