// oxlint-disable effecttsgo/async-function -- These DOM-facing APIs must retain their native Promise contracts; migrating their error channels belongs to #636.
// oxlint-disable effecttsgo/global-fetch -- Native browser requests retain Response, AbortSignal, and Promise rejection contracts; Effect error handling is scoped to #636.
import * as Arr from "effect/Array";
import * as Clock from "effect/Clock";
import * as Effect from "effect/Effect";
import * as Opt from "effect/Option";
import * as Random from "effect/Random";
import * as Schema from "effect/Schema";
import * as Str from "effect/String";
import { attachmentIDs, clearAttachments } from "../../components/chatcomposer/attachments";
import { cloneElement } from "../clone";
import { decode } from "../decode";
import { required } from "../required";
const LinkSchema = Schema.Struct({
  url: Schema.String,
  label: Schema.String,
  source: Schema.optionalKey(Schema.String),
});
const EntrySchema = Schema.Struct({
  attachments: Schema.optionalKey(Schema.Array(Schema.String)),
  id: Schema.String,
  model: Schema.optionalKey(Schema.String),
  text: Schema.String,
  body: Schema.optionalKey(Schema.String),
  links: Schema.optionalKey(Schema.Array(LinkSchema)),
  thread: Schema.optionalKey(Schema.String),
  error: Schema.optionalKey(Schema.String),
  refused: Schema.optionalKey(Schema.Boolean),
});
const SavedSchema = Schema.Struct({
  draft: Schema.optionalKey(Schema.String),
  pending: Schema.optionalKey(Schema.Array(EntrySchema)),
});
type Link = { url: string; label: string; source?: string | undefined };
type Entry = {
  attachments?: readonly string[] | undefined;
  id: string;
  model?: string | undefined;
  text: string;
  body?: string | undefined;
  links?: readonly Link[] | undefined;
  thread?: string | undefined;
  error?: string | undefined;
  refused?: boolean | undefined;
};
type Saved = { draft: string; pending: Entry[] };
type Item = {
  attachments?: readonly string[] | undefined;
  bubble: HTMLElement;
  id: string;
  model: string;
  prompt: string;
  scope: string;
  body?: string | undefined;
  links?: readonly Link[] | undefined;
  thread?: string | undefined;
  waiting?: boolean;
  sending?: boolean;
  enqueue?: boolean;
  resent?: boolean;
};
type State = {
  pending: Item[];
  running: boolean;
  form: HTMLFormElement | null;
  threadID: string | undefined;
  observer?: MutationObserver;
  first?: string;
  queuing?: number;
  followups?: Promise<void>;
};
type Found = Record<string, { status?: string; queued?: boolean } | undefined>;
declare global {
  interface Window {
    housecatCodexSubmit?: (form: HTMLFormElement, text?: string) => void;
    housecatCodexSendable?: (chat: HTMLElement | null) => boolean;
    housecatCodexRetryMs?: number;
  }
}
const retryWindow = () => window.housecatCodexRetryMs ?? 15_000;
const scopeOf = (chat: HTMLElement) => "housecat.chat." + (chat.dataset.chatUser || "anonymous") + ".";
const storageKey = (chat: HTMLElement, threadID?: string) => scopeOf(chat) + (threadID ? "thread." + threadID : "new");
export function outgoing(_hook: HTMLElement, _swapped: boolean): void {
  (() => {
    if (window.housecatCodexSubmit) return;
    const states = new WeakMap<HTMLElement, State>();
    const items = new Map<string, Item>();
    const bubbles = new WeakMap<HTMLElement, Item>();
    const mounted = new WeakMap<Element, string>();
    // retryWindow is how long deliver keeps trying a send nothing answered;
    // a test sets window.housecatCodexRetryMs to shorten it.

    const asking = new Set<string>();
    const polls = new WeakMap<HTMLElement, number>();

    function load(key: string): Saved {
      try {
        const saved = decode(SavedSchema)(JSON.parse(window.sessionStorage.getItem(key) || "{}"));
        return {
          draft: typeof saved.draft === "string" ? saved.draft : "",
          pending: saved.pending ? [...saved.pending] : [],
        };
      } catch {
        return { draft: "", pending: [] };
      }
    }
    function store(key: string, saved: Saved) {
      try {
        if (saved.draft || saved.pending.length) {
          window.sessionStorage.setItem(key, JSON.stringify(saved));
        } else window.sessionStorage.removeItem(key);
      } catch {}
    }
    function storedKeys(scope: string) {
      try {
        return Arr.filter(
          Array.from({ length: window.sessionStorage.length }, (_, i) => window.sessionStorage.key(i)),
          (key): key is string => key !== null && Str.startsWith(scope)(key),
        );
      } catch {
        return [];
      }
    }
    function update(scope: string, id: string, change: (entry: Entry) => Entry | null) {
      for (const key of storedKeys(scope)) {
        const saved = load(key);
        const index = Opt.getOrElse(
          Arr.findFirstIndex(saved.pending, (entry) => entry.id === id),
          () => -1,
        );
        if (index < 0) continue;
        const entry = change(saved.pending[index]);
        if (entry) saved.pending[index] = entry;
        else saved.pending.splice(index, 1);
        store(key, saved);
        return true;
      }
      return false;
    }
    function moveTo(scope: string, id: string, key: string) {
      const selected: { entry: Entry | null } = { entry: null };
      update(scope, id, (found) => {
        selected.entry = found;
        return null;
      });
      if (!selected.entry) return;
      const saved = load(key);
      saved.pending.push(selected.entry);
      store(key, saved);
    }
    function keep(chat: HTMLElement, item: Item) {
      const key = storageKey(chat, chat.dataset.selectedThread);
      const saved = load(key);
      saved.pending.push({
        attachments: item.attachments,
        id: item.id,
        model: item.model,
        text: item.prompt,
        body: item.body,
        links: item.links,
        thread: item.thread,
      });
      store(key, saved);
    }
    function forget(item: Item) {
      update(item.scope, item.id, () => null);
      items.delete(item.id);
    }
    function drop(chat: HTMLElement, id: string) {
      items.get(id)?.bubble.remove();
      update(scopeOf(chat), id, () => null);
      items.delete(id);
    }
    function failure(item: Item) {
      const target = item.bubble.querySelector<HTMLElement>("[data-outgoing-state]");
      const refused = !!required(target).querySelector<HTMLElement>("[data-chat-send-error]");
      update(item.scope, item.id, (entry) => ({
        ...entry,
        error: Str.trim(required(target).textContent),
        refused,
      }));
    }
    function hold(item: Item, failed: boolean) {
      item.waiting = !failed;
      required(item.bubble.querySelector<HTMLElement>("[data-outgoing-dismiss]")).hidden = !failed;
      required(item.bubble.querySelector<HTMLElement>("[data-outgoing-action=resend]")).hidden = !failed;
    }
    const fail = (item: Item) => hold(item, true);
    function bubbleFor(chat: HTMLElement, prompt: string, id: string, links?: readonly Link[]) {
      const bubble = cloneElement(
        required(
          required(chat.querySelector<HTMLTemplateElement>("[data-outgoing-template=message]"))
            .content.firstElementChild,
        ),
      );
      const text = bubble.querySelector<HTMLElement>("[data-slot=user-message-text]");
      required(text).textContent = prompt;
      required(required(text).parentElement).setAttribute("data-outgoing-message", id);
      chat.dispatchEvent(
        new CustomEvent("codex:render-outgoing-links", {
          bubbles: true,
          detail: { bubble, prompt, links },
        }),
      );
      return bubble;
    }
    function track(item: Item) {
      items.set(item.id, item);
      bubbles.set(item.bubble, item);
      return item;
    }
    function place(chat: HTMLElement, item: Item) {
      required(chat.querySelector<HTMLElement>("[data-outgoing-list]")).append(item.bubble);
      required(window.htmx).process(item.bubble);
    }
    // show draws a saved message: as a send in flight looks while waiting, or
    // as not sent, with the error it failed with, Resend and Delete.
    function show(chat: HTMLElement, entry: Entry, waiting: boolean) {
      let item = items.get(entry.id);
      if (!item || !item.bubble.isConnected) {
        item = track({
          attachments: entry.attachments,
          bubble: bubbleFor(chat, entry.text, entry.id, entry.links),
          id: entry.id,
          model: entry.model || "",
          prompt: entry.text,
          body: entry.body,
          links: entry.links,
          scope: scopeOf(chat),
          thread: entry.thread,
        });
        place(chat, item);
      }
      const target = item.bubble.querySelector<HTMLElement>("[data-outgoing-state]");
      if (waiting) required(target).textContent = "Sending…";
      else if (entry.refused === true) {
        const refusal = cloneElement(
          required(
            required(chat.querySelector<HTMLTemplateElement>("[data-outgoing-template=send-error]"))
              .content.firstElementChild,
          ),
        );
        refusal.textContent = entry.error ?? "";
        required(target).replaceChildren(refusal);
      } else {
        required(target).textContent = entry.error || "Not sent.";
      }
      hold(item, !waiting);
    }
    function mount(chat: HTMLElement) {
      const field = chat.querySelector<HTMLInputElement>("[data-chat-form] [data-prompt-field]");
      if (!field) return;
      const key = storageKey(chat, chat.dataset.selectedThread);
      if (mounted.get(field) === key) return;
      const first = !mounted.has(field);
      mounted.set(field, key);
      const draft = load(key).draft;
      if ((first && !draft) || field.value === draft) return;
      field.value = draft;
      field.dispatchEvent(new Event("input", { bubbles: true }));
    }
    // delivered is every client id the chat shows the server has: its
    // transcript's messages and its queue's rows.
    function delivered(chat: HTMLElement) {
      return new Set(
        Array.from(
          chat.querySelectorAll<HTMLElement>(
            "[data-chat-entries] [data-outgoing-message], [data-chat-queue] [data-outgoing-message]",
          ),
          (el) => el.dataset.outgoingMessage,
        ),
      );
    }
    let restoring = false;
    function restore(chat: HTMLElement) {
      const thread = chat.dataset.selectedThread || "";
      const key = storageKey(chat, thread);
      const saved = load(key);
      if (
        restoring
        || !saved.pending.length
        || !chat.querySelector<HTMLElement>("[data-outgoing-list]")
      ) {
        return;
      }
      let moved = false;
      restoring = true;
      try {
        const known = delivered(chat);
        const unknown: Entry[] = [];
        const kept = Arr.filter(saved.pending, (entry) => {
          const item = items.get(entry.id);
          if (known.has(entry.id)) {
            item?.bubble.remove();
            items.delete(entry.id);
            return false;
          }
          if (!item || !(item.sending === true || item.bubble.isConnected)) unknown.push(entry);
          return true;
        });
        if (kept.length !== saved.pending.length) store(key, { ...saved, pending: kept });
        if (thread) ask(chat, thread, unknown);
        else moved = restoreNewChat(chat, unknown);
      } finally {
        restoring = false;
      }
      if (moved) restore(chat);
    }
    // restoreNewChat accounts for a new chat's saved messages without asking
    // the server, so the panel is usable at once. A message that started a chat
    // whose answer the page lost is found by that chat's row in the chat list,
    // which carries the message's client id as the chat's own
    // (data-client-thread-id; see ClientThreadID), and so is a message sent
    // after it, by the first message's client id it keeps: each such message
    // moves to its chat, and the panel moves to the chat too, taking the new
    // chat's draft and other messages with it, whose own restore then accounts
    // for them. With no row to find -- the list not on the page, or the chat
    // not in its first page -- each is drawn as not sent, and its Resend is
    // answered with the chat the server's record of it names, not sent again.
    // Returns whether the panel moved.
    function restoreNewChat(chat: HTMLElement, entries: Entry[]) {
      const state = initialize(chat);
      let started = "";
      for (const entry of entries) {
        const row = document.querySelector<HTMLElement>(
          '[data-client-thread-id="' + CSS.escape(entry.thread || entry.id) + '"]',
        );
        const thread = row ? row.dataset.chatId || row.dataset.threadId || "" : "";
        if (!thread || state.running || state.pending.length) continue;
        moveTo(scopeOf(chat), entry.id, storageKey(chat, thread));
        started = thread;
      }
      if (started) {
        state.threadID = started;
        connect(chat, started);
        return true;
      }
      for (const entry of entries) show(chat, entry, false);
      return false;
    }
    // askLimit is how many messages one question to data-sends-url asks
    // about, the server's cap; more are asked in turns.
    const askLimit = 50;
    function ask(chat: HTMLElement, thread: string, entries: Entry[]) {
      entries = Arr.filter(entries, (entry) => !asking.has(entry.id));
      if (!entries.length) return;
      if (entries.length > askLimit) {
        for (let i = 0; i < entries.length; i += askLimit) {
          ask(chat, thread, entries.slice(i, i + askLimit));
        }
        return;
      }
      Arr.forEach(entries, (entry) => asking.add(entry.id));
      const url = chat.dataset.sendsUrl;
      const query = new URLSearchParams({ thread });
      Arr.forEach(entries, (entry) => query.append("id", entry.id));
      const answer = url
        ? fetch(url + "?" + query.toString(), { headers: { Accept: "application/json" } }).then(
          (response) => (response.ok ? response.json() : {}),
        )
        : Promise.resolve({});
      void answer
        .catch(() => ({}))
        .then((found) => {
          if (chat.isConnected && (chat.dataset.selectedThread || "") === thread) {
            settle(chat, entries, found);
          }
        })
        .finally(() => Arr.forEach(entries, (entry) => asking.delete(entry.id)));
    }
    function settle(chat: HTMLElement, entries: Entry[], found: Found) {
      const known = delivered(chat);
      const saved = new Set(
        Arr.map(load(storageKey(chat, chat.dataset.selectedThread)).pending, (entry) => entry.id),
      );
      for (const entry of entries) {
        if (!saved.has(entry.id) || items.get(entry.id)?.sending === true) continue;
        const state = found[entry.id] ?? { status: "missing" };
        if (known.has(entry.id) || state.status === "sent" || state.status === "cancelled") {
          drop(chat, entry.id);
        } else {
          show(
            chat,
            entry,
            state.queued === true || state.status === "pending" || state.status === "sending",
          );
        }
      }
      watch(chat);
    }
    // watch asks again, every few seconds, about messages drawn as still being
    // sent, until each is in the chat or known not to be.
    function watch(chat: HTMLElement) {
      const waiting = () =>
        Arr.filter(load(storageKey(chat, chat.dataset.selectedThread)).pending, (entry) => {
          const item = items.get(entry.id);
          return item?.waiting === true && item.sending !== true && chat.contains(item.bubble);
        });
      if (polls.has(chat) || !waiting().length) return;
      polls.set(
        chat,
        window.setTimeout(() => {
          polls.delete(chat);
          const entries = waiting();
          if (chat.isConnected && chat.dataset.selectedThread && entries.length) {
            ask(chat, chat.dataset.selectedThread, entries);
          }
        }, 3000),
      );
    }
    function reconcile(chat: HTMLElement) {
      const turns = new Set(
        Array.from(
          chat.querySelectorAll<HTMLElement>("[data-chat-scroll] section[data-turn-id]"),
          (el) => el.dataset.turnId,
        ),
      );
      for (const row of chat.querySelectorAll<HTMLElement>("[data-queued-message]")) {
        const hidden = row.dataset.status === "sent"
          && turns.has(row.dataset.threadId + "/" + row.dataset.turnId);
        if (row.hidden !== hidden) row.hidden = hidden;
      }
      const known = delivered(chat);
      for (const bubble of chat.querySelectorAll<HTMLElement>("[data-outgoing]")) {
        const item = bubbles.get(bubble);
        const result = bubble.querySelector<HTMLElement>("[data-outgoing-result]");
        const queued = !!result
          && result.dataset.queueId !== "0"
          && Arr.some(
            Array.from(chat.querySelectorAll<HTMLElement>("[data-queued-message]")),
            (row) => row.dataset.queueId === result.dataset.queueId,
          );
        if (
          (item && known.has(item.id))
          || queued
          || (result && turns.has(result.dataset.threadId + "/" + result.dataset.turnId))
        ) {
          bubble.remove();
          if (item) forget(item);
          continue;
        }
        if (!result) continue;
        const pending = bubble.querySelector<HTMLElement>("[data-outgoing-pending]");
        if (
          result.dataset.queueId === "0"
          && result.dataset.turnId
          && !required(pending).firstElementChild
        ) {
          required(pending).replaceChildren(
            required(
              chat.querySelector<HTMLTemplateElement>("[data-pending-template]"),
            ).content.cloneNode(true),
          );
        }
      }
    }
    function initialize(chat: HTMLElement) {
      let state = states.get(chat);
      const form = chat.querySelector<HTMLFormElement>("[data-chat-form]");
      if (state?.form === form && state.threadID === chat.dataset.selectedThread) return state;
      state?.observer?.disconnect();
      state = {
        pending: [],
        running: false,
        form,
        threadID: chat.dataset.selectedThread,
      };
      states.set(chat, state);
      const observer = new MutationObserver(() => {
        if (!chat.isConnected) {
          observer.disconnect();
          return;
        }
        reconcile(chat);
        restore(chat);
      });
      state.observer = observer;
      observer.observe(chat, { childList: true, subtree: true });
      return state;
    }
    function connect(chat: HTMLElement, threadID: string | undefined) {
      if (chat.dataset.selectedThread === threadID) return;
      const from = storageKey(chat, chat.dataset.selectedThread);
      const to = storageKey(chat, threadID);
      const moving = load(from);
      if (!chat.dataset.selectedThread && (moving.draft || moving.pending.length)) {
        const staying = load(to);
        store(to, {
          draft: moving.draft || staying.draft,
          pending: staying.pending.concat(moving.pending),
        });
        store(from, { draft: "", pending: [] });
      }
      if (!chat.dataset.selectedThread) {
        chat.dispatchEvent(
          new CustomEvent("codex:context-adopt", { bubbles: true, detail: { threadID } }),
        );
      }
      chat.dataset.selectedThread = threadID ?? "";
      const field = chat.querySelector<HTMLInputElement>("[data-chat-form] [data-prompt-field]");
      if (field) mounted.set(field, to);
      const placeholder = chat.dataset.threadPlaceholder;
      const fill = (template?: string) =>
        template
          ? Arr.join(Str.split(placeholder ?? "")(template), encodeURIComponent(threadID ?? ""))
          : "";
      const form = chat.querySelector<HTMLFormElement>("[data-chat-form]");
      const submitURL = fill(chat.dataset.submitUrlTemplate);
      required(form).setAttribute("action", submitURL);
      required(form).setAttribute("hx-post", submitURL);
      const transcript = chat.querySelector<HTMLElement>('[hx-swap="innerMorph"]');
      const eventsURL = fill(chat.dataset.eventsUrlTemplate);
      if (eventsURL) required(transcript).setAttribute("hx-sse:connect", eventsURL);
      else required(transcript).removeAttribute("hx-sse:connect");
      required(window.htmx).process(required(transcript));
      const queue = chat.querySelector<HTMLElement>("[data-chat-queue]");
      const queueURL = fill(chat.dataset.queueUrlTemplate);
      if (queueURL) required(queue).setAttribute("hx-get", queueURL);
      else required(queue).removeAttribute("hx-get");
      required(window.htmx).process(required(queue));
      chat.dispatchEvent(
        new CustomEvent(chat.dataset.namespace + ":thread-created", {
          bubbles: true,
          detail: { threadID },
        }),
      );
      chat.dispatchEvent(new CustomEvent("codex:context-changed", { bubbles: true }));
    }
    function adopt(chat: HTMLElement, state: State, threadID: string | undefined) {
      const to = storageKey(chat, threadID);
      for (const waiting of state.pending) {
        if (items.get(waiting.id) === waiting) moveTo(waiting.scope, waiting.id, to);
      }
    }
    // A send the page cuts off by going away has not failed: its saved copy
    // is left as it was, for the next page to ask the server about. That next
    // page may be this one, back out of the browser's cache, so a bubble left
    // without an answer then goes, and restore asks about its message.
    let leaving = false;
    window.addEventListener("pagehide", () => {
      leaving = true;
    });
    window.addEventListener("pageshow", (event) => {
      leaving = false;
      if (!event.persisted) return;
      for (const chat of document.querySelectorAll<HTMLElement>("[data-chat]")) {
        for (const bubble of chat.querySelectorAll<HTMLElement>("[data-outgoing]")) {
          const item = bubbles.get(bubble);
          if (
            !item
            || item.sending === true
            || bubble.querySelector<HTMLElement>("[data-outgoing-result]")
          ) {
            continue;
          }
          bubble.remove();
          items.delete(item.id);
        }
        restore(chat);
      }
    });
    function current(chat: HTMLElement, state: State) {
      return (
        chat.isConnected
        && states.get(chat) === state
        && state.form === chat.querySelector<HTMLFormElement>("[data-chat-form]")
        && state.threadID === chat.dataset.selectedThread
      );
    }
    async function deliver(chat: HTMLElement, state: State, item: Item, enqueue?: boolean) {
      if (!current(chat, state)) return;
      if (!state.threadID && !item.thread) state.first = item.id;
      const target = item.bubble.querySelector<HTMLElement>("[data-outgoing-state]");
      required(target).setAttribute("hx-status:4xx", '{"swap":"innerHTML"}');
      required(target).setAttribute("hx-status:5xx", '{"swap":"innerHTML"}');
      item.sending = true;
      item.waiting = false;
      // A send nothing answered -- the gateway's page while the app restarts,
      // which request-alert.js refuses to draw, or no answer at all -- is sent
      // again under the same client id, which the server answers once, for
      // retryWindow, backing off between tries. The bubble keeps saying
      // Sending… meanwhile; only a send still unanswered after that is shown
      // as not confirmed. An answer of any kind, the server's refusal
      // included, ends it.
      const deadline = Effect.runSync(Clock.currentTimeMillis) + retryWindow();
      for (let attempt = 0;; attempt++) {
        let threw = false;
        try {
          // oxlint-disable-next-line eslint/no-await-in-loop -- This loop intentionally sequences requests or retries; concurrent iterations change the UI contract.
          await required(window.htmx).ajax("POST", required(required(state.form).getAttribute("action")), {
            source: target,
            target,
            swap: "innerHTML",
            values: {
              prompt: item.body ?? item.prompt,
              attachments: Arr.join(item.attachments ?? [], ","),
              contexts: JSON.stringify(
                Arr.map(item.links ?? [], (link) => {
                  if (link.source !== "automatic") return link;
                  const url = new URL(link.url, location.origin);
                  return Object.assign(link, { url: url.pathname + url.search + url.hash });
                }),
              ),
              model: item.model,
              optimistic: "true",
              enqueue: enqueue === true ? "true" : "false",
              client_id: item.id,
              client_thread_id: item.thread || "",
              resend: item.resent === true ? "true" : "false",
            },
          });
        } catch {
          threw = true;
        }
        const unanswered = !required(target).querySelector<HTMLElement>(
          "[data-outgoing-result], [data-chat-send-error]",
        )
          && (threw || required(target).textContent === "Sending…");
        const wait = Math.min(
          1000 * 2 ** attempt,
          deadline - Effect.runSync(Clock.currentTimeMillis),
        );
        if (
          !unanswered
          || wait <= 0
          || leaving
          || !current(chat, state)
          || !item.bubble.isConnected
        ) {
          if (threw) {
            required(target).textContent =
              "Delivery could not be confirmed. Check the transcript before sending again.";
          }
          break;
        }
        // oxlint-disable-next-line eslint/no-await-in-loop -- This loop intentionally sequences requests or retries; concurrent iterations change the UI contract.
        await Effect.runPromise(Effect.sleep(wait));
        // oxlint-disable-next-line typescript/no-unnecessary-condition -- Navigation and transcript swaps can change both values during the awaited retry delay.
        if (leaving || !current(chat, state) || !item.bubble.isConnected) break;
      }
      item.sending = false;
      const result = required(target).querySelector<HTMLElement>("[data-outgoing-result]");
      chat.dispatchEvent(
        new CustomEvent("codex:context-changed", {
          bubbles: true,
          detail: { id: item.id, delivered: !!result },
        }),
      );
      if (leaving) return;
      const started = state.threadID
        ? ""
        : required(target).querySelector<HTMLElement>("[data-outgoing-thread]")?.dataset
          .outgoingThread || "";
      if (result) forget(item);
      else {
        if (
          required(target).textContent === "Sending…"
          || !Str.trim(required(target).textContent)
        ) {
          required(target).textContent = "Delivery could not be confirmed. Check the transcript before sending again.";
        }
        failure(item);
      }
      if (!current(chat, state)) {
        if (!result && started) {
          moveTo(item.scope, item.id, storageKey(chat, started));
          adopt(chat, state, started);
        }
        if (!result) document.querySelectorAll<HTMLElement>("[data-chat]").forEach(restore);
        else if (!state.threadID) adopt(chat, state, result.dataset.threadId);
        return;
      }
      if (result) {
        state.threadID = result.dataset.threadId;
        connect(chat, result.dataset.threadId);
        if (result.dataset.queueId !== "0") {
          required(window.htmx).trigger(
            required(chat.querySelector<HTMLElement>("[data-chat-queue]")),
            "codex:queued",
          );
        }
        reconcile(chat);
      } else {
        fail(item);
        if (started) {
          state.threadID = started;
          connect(chat, started);
        } else if (!state.threadID) {
          for (const next of state.pending.splice(0)) {
            required(next.bubble.querySelector<HTMLElement>("[data-outgoing-state]")).textContent =
              "Not sent. Start a chat before sending this instruction.";
            failure(next);
            fail(next);
          }
        }
      }
    }
    async function drain(chat: HTMLElement, state: State) {
      if (state.running) return;
      state.running = true;
      try {
        while (state.pending.length && current(chat, state)) {
          const item = state.pending.shift();
          // oxlint-disable-next-line eslint/no-await-in-loop -- This loop intentionally sequences requests or retries; concurrent iterations change the UI contract.
          await deliver(chat, state, required(item), required(item).enqueue);
        }
      } finally {
        state.running = false;
      }
    }
    // A message sent into a new chat while its first is still with the server
    // goes at once rather than waiting its turn here, naming its chat by the
    // first message's client id, which it keeps saved: the server queues it
    // in the chat the first one starts, so it is the server's even if this
    // page goes away, and a Resend or a restore finds that chat by it too.
    // Messages typed one after another reach the server in that order, which
    // is the order it queues them in.
    function send(chat: HTMLElement, state: State, item: Item) {
      item.enqueue = state.running;
      if (state.threadID && (state.running || Boolean(state.queuing))) {
        state.queuing = (state.queuing ?? 0) + 1;
        state.followups = (state.followups ?? Promise.resolve())
          .then(() => deliver(chat, state, item, true))
          .finally(() => {
            state.queuing = (state.queuing ?? 1) - 1;
          });
      } else if (!state.threadID && state.running && state.first && state.first !== item.id) {
        item.thread ||= state.first;
        update(item.scope, item.id, (entry) => ({ ...entry, thread: item.thread }));
        void deliver(chat, state, item, true);
      } else {
        state.pending.push(item);
        void drain(chat, state);
      }
    }
    // newID is a version 4 UUID, the one form of client id the server keeps;
    // any other is replaced with one of its own, which the page can never
    // match. Without crypto.randomUUID (an insecure context, an older browser)
    // it is built from random bytes in the same form.
    function newID() {
      const crypto: Partial<Crypto> = window.crypto;
      if (crypto.randomUUID) return crypto.randomUUID();
      const bytes = new Uint8Array(16);
      if (crypto.getRandomValues) crypto.getRandomValues(bytes);
      else for (let i = 0; i < 16; i++) bytes[i] = Math.floor(Effect.runSync(Random.next) * 256);
      bytes[6] = (bytes[6] & 0x0F) | 0x40;
      bytes[8] = (bytes[8] & 0x3F) | 0x80;
      const hex = Arr.join(
        Array.from(bytes, (b) => Str.padStart(2, "0")(b.toString(16))),
        "",
      );
      return Arr.join(
        [
          Str.slice(0, 8)(hex),
          Str.slice(8, 12)(hex),
          Str.slice(12, 16)(hex),
          Str.slice(16, 20)(hex),
          Str.slice(20)(hex),
        ],
        "-",
      );
    }
    // text is a message sent from outside the input, a starter prompt's; the
    // input and its draft stay as they are.
    window.housecatCodexSubmit = (form, text) => {
      const chat = form.closest<HTMLElement>("[data-chat]");
      const input = form.querySelector<HTMLInputElement>("[data-prompt-field]");
      let prompt = Str.trim(text ?? required(input).value);
      if (!prompt) return;
      const body = prompt;
      const context = form.querySelector<HTMLElement>("[data-slot=chat-page-context]");
      if (context?.hidden === false && context.dataset.url) {
        prompt += "\n\nCurrent page: " + required(context.querySelector<HTMLAnchorElement>("a")).href;
      }
      const state = initialize(required(chat));
      const links: Link[] = [];
      form.dispatchEvent(
        new CustomEvent("codex:collect-outgoing-links", {
          bubbles: true,
          detail: { links, includeDraft: text === undefined },
        }),
      );
      const id = newID();
      const attachments = text === undefined ? attachmentIDs(form) : [];
      const item = track({
        attachments,
        bubble: bubbleFor(required(chat), prompt, id, links),
        body,
        links,
        id,
        model: form.querySelector<HTMLInputElement>("[name=model]")?.value || "",
        prompt,
        scope: scopeOf(required(chat)),
      });
      keep(required(chat), item);
      place(required(chat), item);
      required(chat).dispatchEvent(
        new CustomEvent("codex:context-pending", { bubbles: true, detail: { id: item.id, links } }),
      );
      if (text === undefined) {
        clearAttachments(form);
        required(input).value = "";
        required(input).dispatchEvent(new Event("input", { bubbles: true }));
      }
      required(form.querySelector<HTMLElement>("[data-slot=chat-composer-editor]") ?? input).focus({
        preventScroll: true,
      });
      const scroll = required(chat).querySelector<HTMLElement>("[data-chat-scroll]");
      required(scroll).dataset.follow = "true";
      required(scroll).scrollTop = required(scroll).scrollHeight;
      send(required(chat), state, item);
    };
    function sendable(chat: HTMLElement | null) {
      const field = chat?.querySelector<HTMLInputElement>("[data-chat-form] [data-prompt-field]");
      return (
        !!field
        && !field.disabled
        && !field.readOnly
        && required(chat).querySelector<HTMLElement>("[data-chat-composer]")?.dataset.unavailable
          !== "true"
      );
    }
    window.housecatCodexSendable = sendable;
    function resend(bubble: HTMLElement) {
      const chat = bubble.closest<HTMLElement>("[data-chat]");
      const item = bubbles.get(bubble);
      if (!chat || !item || !sendable(chat)) return;
      const state = initialize(chat);
      required(bubble.querySelector<HTMLElement>("[data-outgoing-state]")).textContent = "Sending…";
      hold(item, false);
      // From here the send is this page's, not one to ask the server about
      // while it waits its turn behind another. It is sent as a resend, which
      // the server checks against the chat before sending.
      item.waiting = false;
      item.resent = true;
      if (
        !update(item.scope, item.id, (entry) => ({
          id: entry.id,
          model: entry.model,
          text: entry.text,
          body: entry.body,
          links: entry.links,
          thread: entry.thread,
        }))
      ) {
        keep(chat, item);
      }
      send(chat, state, item);
    }
    document.addEventListener("click", (event) => {
      const button = event.target instanceof Element
        ? event.target.closest<HTMLElement>(
          "[data-outgoing] [data-outgoing-dismiss], [data-outgoing] [data-outgoing-action=resend]",
        )
        : null;
      if (!button) return;
      const bubble = button.closest<HTMLElement>("[data-outgoing]");
      if (!button.hasAttribute("data-outgoing-dismiss")) {
        resend(required(bubble));
        return;
      }
      required(bubble).remove();
      const item = bubbles.get(required(bubble));
      if (item) forget(item);
    });
    document.addEventListener("input", (event) => {
      const field = event.target instanceof Element
        ? event.target.closest<HTMLInputElement>("[data-chat-form] [data-prompt-field]")
        : null;
      const chat = field?.closest<HTMLElement>("[data-chat]");
      if (!chat || !field) return;
      const key = storageKey(chat, chat.dataset.selectedThread);
      if (mounted.has(field) && mounted.get(field) !== key) return;
      const saved = load(key);
      saved.draft = Str.trim(field.value) ? field.value : "";
      store(key, saved);
    });
    required(window.htmx).registerExtension("codex-outgoing", {
      htmx_after_process(element: HTMLElement) {
        const inside = element.closest<HTMLElement>("[data-chat]");
        for (
          const chat of inside
            ? [inside]
            : element.querySelectorAll<HTMLElement>("[data-chat]")
        ) {
          initialize(chat);
          mount(chat);
          reconcile(chat);
          restore(chat);
        }
      },
    });
  })();
}
