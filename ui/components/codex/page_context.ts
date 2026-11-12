import * as Schema from "effect/Schema";

import { setAttributeIfChanged, setTextIfChanged } from "../../dom/dom";
import { forgetStorage, isAborted, readJSON, readStorage, writeStorage } from "../../dom/failures";
import { registerComponent } from "../../dom/lifecycle";
import { reportError } from "../../dom/runtime";
import { readDecoded } from "../../dom/wire";
import * as Wire from "../../schemas/schemas.gen";

// The chat's own events (codex.Outgoing's script, and the HX-Trigger header
// for the baseline), which bubble from the chat. A context-changed without a
// message says only that the chat changed. The baseline is the server's JSON,
// a codex.ContextBaseline, decoded where it is read.
declare global {
  interface GlobalEventHandlersEventMap {
    "codex:context-adopt": CustomEvent<{ threadID: string }>;
    "codex:context-baseline": CustomEvent<unknown>;
    "codex:context-changed": CustomEvent<{ id?: string; delivered?: boolean } | null>;
    "codex:context-pending": CustomEvent<{ id: string; links: { url: string; source?: string }[] }>;
  }
}

const chips = new Set<HTMLElement>();
const timers = new WeakMap<HTMLElement, ReturnType<typeof setTimeout>>();
type ChipParts = { link: HTMLAnchorElement; label: HTMLElement; detail: HTMLElement };
const parts = new WeakMap<HTMLElement, ChipParts>();
const selector = '[data-slot="chat-page-context"]';
// PageState is what the page context keeps in sessionStorage for a chat:
// the page it was last shown on, and whether it was dismissed there. Only
// this script writes it.
const PageState = Schema.Struct({ dismissed: Schema.Boolean, url: Schema.String });
type PageState = typeof PageState.Type;
const memory = new Map<string, PageState>();
const checked = new WeakMap<HTMLElement, string>();
const requests = new WeakMap<HTMLElement, AbortController>();
type ContextState = { baseline: string; pending: { id: string; key: string }[] };
const lastContexts = new Map<string, ContextState>();
const baselines = new WeakMap<HTMLElement, string>();

function pageURL(): string {
  const url = new URL(location.href);
  if (url.pathname !== "/mail" && !url.pathname.startsWith("/mail/")) return "";
  url.searchParams.delete("chat");
  url.searchParams.delete("_partial");
  url.searchParams.sort();
  return url.pathname + url.search + url.hash;
}

function key(chip: HTMLElement, thread?: string): string {
  const chat = chip.closest<HTMLElement>("[data-chat]");
  return "housecat.chat.page."
    + JSON.stringify([chat?.dataset.chatUser || "anonymous", thread ?? chat?.dataset.selectedThread ?? ""]);
}

function save(scope: string, state: PageState): void {
  memory.set(scope, state);
  writeStorage("session", scope, JSON.stringify(state));
}

function stateFor(chip: HTMLElement, url: string): PageState {
  const scope = key(chip);
  let state = memory.get(scope);
  // Without a stored state, or with one that isn't JSON, the current page
  // starts afresh.
  if (!state) {
    const stored = readJSON(readStorage("session", scope));
    if (Schema.is(PageState)(stored)) state = stored;
  }
  if (!state || state.url !== url) {
    state = { url, dismissed: false };
    save(scope, state);
  } else {
    memory.set(scope, state);
  }
  return state;
}

function title(chip: HTMLElement, label: string, detail: string): void {
  const elements = parts.get(chip);
  if (!elements) return;
  setTextIfChanged(elements.label, label);
  setTextIfChanged(elements.detail, detail ? " · " + detail : "");
}

function cancelPreview(chip: HTMLElement): void {
  requests.get(chip)?.abort();
  clearTimeout(timers.get(chip));
  requests.delete(chip);
  timers.delete(chip);
}

async function enrich(chip: HTMLElement, url: string, thread: string): Promise<void> {
  cancelPreview(chip);
  const controller = new AbortController();
  requests.set(chip, controller);
  const timeout = setTimeout(() => controller.abort(), 2000);
  timers.set(chip, timeout);
  try {
    const response = await fetch("/codex/link-preview?" + new URLSearchParams({ url, thread }).toString(), {
      signal: controller.signal,
    });
    if (!response.ok) return;
    const label = readDecoded(
      Wire.LinkPreviewOut,
      { endpoint: "/codex/link-preview", operation: "chat.page-context-label" },
      await response.json(),
    );
    if (
      !controller.signal.aborted && chip.isConnected && chip.dataset.url === url
      && (chip.closest<HTMLElement>("[data-chat]")?.dataset.selectedThread || "") === thread
      && requests.get(chip) === controller
    ) {
      title(chip, label.label, label.context);
      setAttributeIfChanged(chip, "data-context-key", label.key);
    }
  } catch (failure) {
    // The readable route label stays, whatever stopped the request.
    if (!isAborted(failure)) reportError(failure, { operation: "chat.page-context-label" });
  } finally {
    clearTimeout(timeout);
    if (requests.get(chip) === controller) {
      requests.delete(chip);
      timers.delete(chip);
    }
  }
}

function updateChip(chip: HTMLElement): void {
  if (!chip.isConnected) return;
  const link = chip.querySelector<HTMLAnchorElement>("a");
  const label = chip.querySelector<HTMLElement>('[data-slot="chat-page-context-label"]');
  const detail = chip.querySelector<HTMLElement>('[data-slot="chat-page-context-detail"]');
  if (!link || !label || !detail) {
    cancelPreview(chip);
    parts.delete(chip);
    checked.delete(chip);
    return;
  }
  const previous = parts.get(chip);
  const replaced = previous?.link !== link || previous.label !== label || previous.detail !== detail;
  if (replaced) {
    parts.set(chip, { link, label, detail });
    checked.delete(chip);
  }
  const url = pageURL();
  const state = stateFor(chip, url);
  const open = !!new URL(location.href).searchParams.get("chat");
  const thread = chip.closest<HTMLElement>("[data-chat]")?.dataset.selectedThread || "";
  const signature = JSON.stringify([url, thread]);
  const changed = checked.get(chip) !== signature;
  const scope = key(chip);
  const baseline = chip.dataset.lastContext || "";
  if (!lastContexts.has(scope)) lastContexts.set(scope, { baseline, pending: [] });
  const hints = lastContexts.get(scope)!;
  if (baselines.get(chip) !== baseline) {
    hints.baseline = baseline;
    baselines.set(chip, baseline);
  }
  if (changed) {
    cancelPreview(chip);
    checked.set(chip, signature);
    if (url) void enrich(chip, url, thread);
  }
  setAttributeIfChanged(chip, "data-dismissed", String(state.dismissed));
  const latest = hints.pending.at(-1)?.key ?? hints.baseline;
  const hidden = !url || state.dismissed || !open || contextKey(url) === latest;
  if (chip.hidden !== hidden) chip.hidden = hidden;
  if (chip.dataset.url === url && !replaced) return;
  setAttributeIfChanged(chip, "data-url", url);
  setAttributeIfChanged(link, "title", url);
  if (!url) {
    link.removeAttribute("href");
    return;
  }
  const route = new URL(url, location.origin);
  if (route.protocol !== "http:" && route.protocol !== "https:") return;
  link.href = route.href;
  title(
    chip,
    route.pathname === "/mail" || route.pathname === "/mail/" ? "Mail" : "Email thread",
    route.searchParams.get("account") || "",
  );
}
function update(root?: Element): void {
  for (const chip of chips) if (!root || root.contains(chip)) updateChip(chip);
}

document.addEventListener("click", event => {
  const button = event.target instanceof Element
    ? event.target.closest('[data-slot="chat-page-context-remove"]')
    : null;
  const chip = button?.closest<HTMLElement>(selector);
  if (!chip) return;
  save(key(chip), { url: pageURL(), dismissed: true });
  chip.hidden = true;
  chip.dataset.dismissed = "true";
  chip.closest("form")?.querySelector<HTMLElement>('[data-slot="chat-composer-editor"], [data-prompt-field]')?.focus({
    preventScroll: true,
  });
});
for (const name of ["DOMContentLoaded", "htmx:after:settle", "htmx:after:history:update", "mail:chat-context"]) {
  document.addEventListener(name, () => update());
}
// Discovery runs in the mutation microtask, before a newly inserted chip paints.
registerComponent(selector, (chip, signal) => {
  chips.add(chip);
  signal.addEventListener("abort", () => {
    chips.delete(chip);
    cancelPreview(chip);
    checked.delete(chip);
    baselines.delete(chip);
    parts.delete(chip);
  }, { once: true });
  updateChip(chip);
});
registerComponent("[data-chat]", (chat, signal) => {
  const observer = new MutationObserver(changes => {
    for (const change of changes) {
      if (change.type === "attributes") {
        if (change.target === chat) update(chat);
        else if (change.target instanceof HTMLElement && change.target.matches(selector)) updateChip(change.target);
      } else {
        const chip = change.target instanceof Element ? change.target.closest<HTMLElement>(selector) : null;
        if (chip) updateChip(chip);
        for (const node of change.addedNodes) if (node instanceof Element) update(node);
      }
    }
  });
  observer.observe(chat, {
    attributeFilter: ["data-selected-thread", "data-chat-user", "data-last-context"],
    childList: true,
    subtree: true,
  });
  signal.addEventListener("abort", () => observer.disconnect(), { once: true });
});
window.addEventListener("popstate", () => update());
window.addEventListener("hashchange", () => update());
if (document.readyState !== "loading") update();

function refresh(): void {
  for (const chip of chips) checked.delete(chip);
  update();
}

document.addEventListener("codex:context-pending", event => {
  const chat = event.target;
  if (!(chat instanceof HTMLElement)) return;
  const chip = chat.querySelector<HTMLElement>(selector);
  const { id, links } = event.detail;
  if (chip) {
    for (const link of links) {
      const context = contextKey(link.url);
      if (context) lastContexts.get(key(chip))?.pending.push({ id, key: context });
    }
    update();
  }
});
document.addEventListener("codex:context-changed", event => {
  const detail = event.detail;
  const chip = event.target instanceof Element ? event.target.querySelector<HTMLElement>(selector) : null;
  if (chip && detail?.id) {
    const hints = lastContexts.get(key(chip));
    if (hints) {
      const delivered = hints.pending.filter(hint => hint.id === detail.id);
      const sent = delivered.at(-1);
      if (detail.delivered === true && sent) hints.baseline = sent.key;
      hints.pending = hints.pending.filter(hint => hint.id !== detail.id);
    }
  }
  refresh();
});
document.addEventListener("codex:context-baseline", event => {
  const { thread, key: baseline } = readDecoded(
    Wire.ContextBaseline,
    { endpoint: "/codex/threads/:thread/queue", operation: "codex.context-baseline" },
    event.detail,
  );
  chips.forEach(chip => {
    if (chip.closest<HTMLElement>("[data-chat]")?.dataset.selectedThread !== thread) return;
    const hints = lastContexts.get(key(chip));
    if (hints) hints.baseline = baseline;
  });
  update();
});
window.addEventListener("focus", refresh);

document.addEventListener("codex:context-adopt", event => {
  const chat = event.target;
  if (!(chat instanceof HTMLElement)) return;
  const chip = chat.querySelector<HTMLElement>(selector);
  if (!chip) return;
  const state = stateFor(chip, pageURL());
  const scope = key(chip);
  const next = key(chip, event.detail.threadID);
  save(next, state);
  if (lastContexts.has(scope)) lastContexts.set(next, lastContexts.get(scope)!);
  lastContexts.delete(scope);
  memory.delete(scope);
  forgetStorage("session", scope);
});

function contextKey(href: string): string {
  const url = new URL(href, location.origin);
  if (url.origin !== location.origin && url.origin !== "https://mail.housecat.com") return "";
  if (url.pathname !== "/mail" && !url.pathname.startsWith("/mail/")) return "";
  const account = (url.searchParams.get("account") || "").trim().toLowerCase();
  url.hash = "";
  if (url.pathname === "/mail" || url.pathname === "/mail/") {
    url.pathname = "/mail";
    for (const key of ["chat", "_partial", "back"]) url.searchParams.delete(key);
  } else {
    url.search = "";
  }
  if (account) url.searchParams.set("account", account);
  else url.searchParams.delete("account");
  url.searchParams.sort();
  return url.pathname + url.search;
}

export function currentPageContext(form: HTMLFormElement): { url: string; label: string; source: string } | undefined {
  const chip = form.querySelector<HTMLElement>(selector);
  if (chip) updateChip(chip);
  if (!chip?.dataset.url || chip.dataset.dismissed === "true") return undefined;
  const anchor = chip.querySelector<HTMLAnchorElement>("a");
  if (anchor) return { url: anchor.href, label: anchor.textContent || "Current page", source: "automatic" };
  return undefined;
}
