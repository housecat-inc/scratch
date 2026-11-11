import * as Predicate from "effect/Predicate";

import { setButtonState } from "../components/button/button_state";
import type * as Wire from "../schemas/schemas.gen";
import { matchChord, parseChord } from "./chord";
import { htmxWrite } from "./htmx_write";
import { type Change, createIntentStore, type IntentStore } from "./intents";
import { on } from "./keys";
import { discoverComponents, registerComponent } from "./lifecycle";

const ROOT = '[data-slot="mail-sidebar-table"]';
const ROW = "[data-mail-nav-row]";
const EXPAND_DELAY = 600;
const REORDER_UP = parseChord("alt+ArrowUp");
const REORDER_DOWN = parseChord("alt+ArrowDown");
const CONTROL =
  "a[href],button,input:not([type=hidden]),select,textarea,summary,[tabindex]:not([data-mail-nav-row]):not(input[type=hidden])";
type DragEvent = { item: HTMLElement; oldIndex: number; newIndex: number };
type MoveEvent = { dragged: HTMLElement; related: HTMLElement; willInsertAfter: boolean };
type Sorter = { destroy: () => void; multiDrag?: { destroyGlobal: () => void } };
type SortableBody = HTMLElement & { _appNavSortable?: Sorter };
const pageWindow = window as Window & {
  Sortable?: {
    dragged?: HTMLElement;
    new(el: Element, options: {
      animation: number;
      draggable: string;
      ghostClass: string;
      invertSwap: boolean;
      handle: string;
      onMove: (event: MoveEvent, original: MouseEvent) => boolean;
      onEnd: (event: DragEvent) => void;
    }): Sorter;
  };
  htmx?: {
    swap: (options: { sourceElement: Element; target: Element; swap: string; text: string }) => Promise<unknown> | void;
  };
};
type NavMove = NonNullable<Wire.NavMovesIn>[number];
type State = {
  account: string;
  moves: NavMove[];
  root: HTMLElement;
  body: SortableBody;
  baseline: string;
  busy: boolean;
  queued: boolean;
  reset: boolean;
  settling: boolean;
  moved: string;
  subscription: AbortController;
  bodyEvents: AbortController;
  hoverHeader?: HTMLElement;
  hoverTimer?: ReturnType<typeof setTimeout>;
};
const states = new Map<HTMLElement, State>();
let visibility: IntentStore<boolean> | null = null;
let applyingOrder = false;
const SCOPE = "sidebar";

function visibilityKey(account: string, entry: string): string {
  return JSON.stringify([account, entry]);
}

// unpack is the account and entry a visibilityKey holds.
function unpack(packed: string): [string, string] {
  const pair: unknown = JSON.parse(packed);
  if (!Array.isArray(pair) || pair.length !== 2 || !pair.every(Predicate.isString)) {
    throw new TypeError("not a visibility key: " + packed);
  }
  return [pair[0], pair[1]];
}
function renderVisibility(packed: string, visible: boolean, root?: HTMLElement): void {
  const [account, entry] = unpack(packed);
  const row = Array.from(
    (root ?? document).querySelectorAll<HTMLElement>(root ? "[data-mail-nav-entry]" : ROOT + " [data-mail-nav-entry]"),
  )
    .find((row) => row.dataset.mailNavEntry === entry && row.closest<HTMLElement>(ROOT)?.dataset.account === account);
  if (!row) return;
  const face = visible ? "visible" : "hidden";
  if (row.dataset.visibility !== face) row.dataset.visibility = face;
  const button = row.querySelector('form[action="/settings/mail/nav"] button');
  if (button) {
    if (button.getAttribute("aria-pressed") !== String(visible)) button.setAttribute("aria-pressed", String(visible));
    if (button.getAttribute("data-state") !== face) setButtonState(button, face);
  }
  const field = row.querySelector<HTMLInputElement>('input[name="visible"]');
  if (field && field.value !== String(!visible)) field.value = String(!visible);
  const input = document.getElementById("visible-" + row.dataset.mailNavKey);
  if (input instanceof HTMLInputElement) input.checked = visible;
  document.querySelectorAll<HTMLElement>("[data-mail-sidebar] [data-nav-key]").forEach((link) => {
    if (
      link.dataset.navKey === entry
      && link.closest<HTMLElement>("[data-count-account]")?.dataset.countAccount === account
      && link.hidden === visible
    ) link.hidden = !visible;
  });
}
function renderCurrentVisibility(root?: HTMLElement): void {
  if (!visibility) return;
  (root ?? document).querySelectorAll<HTMLElement>(root ? "[data-mail-nav-entry]" : ROOT + " [data-mail-nav-entry]")
    .forEach((row) => {
      const key = visibilityKey(row.closest<HTMLElement>(ROOT)!.dataset.account!, row.dataset.mailNavEntry!);
      const wanted = visibility!.get(SCOPE, key);
      if (wanted !== undefined) renderVisibility(key, wanted, root);
    });
}
function visibilityStore(): IntentStore<boolean> {
  if (visibility) return visibility;
  // A save the server answers with a 2xx holds what was sent, so the sent
  // values are what it confirms; the answer is the sidebar, which htmx
  // swaps in.
  visibility = createIntentStore<boolean, Change<boolean>[], void>({
    fold: (changes) => changes.length ? changes : null,
    async send(_scope, changes, keepalive) {
      const values = changes.map(change => {
        const [account, key] = unpack(change.key);
        return { account, key, visible: change.value };
      }) satisfies Wire.NavVisibilityChangesIn;
      const data = new URLSearchParams({
        op: "visibility",
        account: values[0].account,
        changes: JSON.stringify(values),
      });
      const answer = await htmxWrite("/settings/mail/nav", data, keepalive);
      if (!answer.ok) throw new Error(answer.text);
    },
    answered() {
      renderCurrentVisibility();
      for (const state of states.values()) announce(state, "Saved the sidebar visibility");
    },
    failed() {
      renderCurrentVisibility();
      for (const state of states.values()) announce(state, "Could not save visibility. Try again.", true);
    },
  });
  return visibility;
}

function isHeader(row: Element | null): boolean {
  return row?.hasAttribute("data-mail-nav-header") === true;
}
function setSectionExpanded(header: HTMLElement, expanded: boolean): void {
  header.setAttribute("aria-expanded", String(expanded));
  header.querySelector("[aria-expanded]")?.setAttribute("aria-expanded", String(expanded));
  for (let entry = header.nextElementSibling; entry && !isHeader(entry); entry = entry.nextElementSibling) {
    if (entry instanceof HTMLElement) entry.hidden = !expanded;
  }
}
function clearHover(s: State): void {
  clearTimeout(s.hoverTimer);
  s.hoverHeader?.removeAttribute("data-drag-opening");
  s.hoverHeader?.style.removeProperty("--mail-section-open-delay");
  delete s.hoverTimer;
  delete s.hoverHeader;
}
function isCollapsed(header: Element): boolean {
  return header.querySelector("[aria-expanded]")?.getAttribute("aria-expanded") === "false";
}
function hoverHeader(s: State, header: HTMLElement): void {
  if (s.hoverHeader === header) return;
  clearHover(s);
  if (!isCollapsed(header)) return;
  s.hoverHeader = header;
  header.style.setProperty("--mail-section-open-delay", `${EXPAND_DELAY}ms`);
  header.setAttribute("data-drag-opening", "");
  s.hoverTimer = setTimeout(() => {
    const dragged = pageWindow.Sortable?.dragged;
    if (header.isConnected && dragged?.parentElement === s.body && dropAllowed(dragged, header, true, true)) {
      setSectionExpanded(header, true);
      header.after(dragged);
    }
    clearHover(s);
  }, EXPAND_DELAY);
}
function dragIntoHeader(s: State, header: HTMLElement, dragged: HTMLElement): void {
  if (isCollapsed(header)) {
    hoverHeader(s, header);
  } else {
    clearHover(s);
    if (header.nextElementSibling !== dragged) header.after(dragged);
  }
}
function rows(s: State): HTMLElement[] {
  return Array.from(s.body.querySelectorAll<HTMLElement>(ROW)).filter((row) => row.hidden === false);
}
function sectionOf(row: Element): string {
  for (let previous = row.previousElementSibling; previous; previous = previous.previousElementSibling) {
    if (isHeader(previous)) return previous instanceof HTMLElement ? previous.dataset.section ?? "" : "";
  }
  return "";
}
function allows(row: HTMLElement, section: string): boolean {
  return (row.dataset.mailNavSections ?? "").split(" ").includes(section);
}
function dropAllowed(row: HTMLElement, related: HTMLElement, after: boolean, allowClosed = false): boolean {
  let below = after ? related.nextElementSibling : related;
  if (below === row) below = row.nextElementSibling;
  if (below?.hasAttribute("data-mail-nav-fixed") === true) return false;
  let section = sectionOf(related);
  if (isHeader(related)) {
    if (after) section = related.dataset.section ?? "";
    else if (!section) return false;
  }
  const header = row.parentElement?.querySelector('[data-mail-nav-header][data-section="' + section + '"]');
  return allows(row, section) && !!header && (allowClosed || !isCollapsed(header));
}
function announce(s: State, text: string, failed = false): void {
  const status = s.root.querySelector<HTMLElement>('[data-slot="mail-settings-status"]');
  if (!status) return;
  status.textContent = text;
  status.classList.toggle("sr-only", !failed);
  status.classList.toggle("text-destructive", failed);
}
function disposeBody(s: State): void {
  clearHover(s);
  s.bodyEvents.abort();
  s.body._appNavSortable?.destroy();
  delete s.body._appNavSortable;
}
function reconcile(s: State, resetBaseline = false): void {
  if (s.subscription.signal.aborted || !s.root.isConnected) return;
  const body = s.root.querySelector<SortableBody>("[data-mail-nav-sortable]");
  if (!body) {
    disposeBody(s);
    return;
  }
  if (s.body !== body || s.bodyEvents.signal.aborted) {
    disposeBody(s);
    s.body = body;
    s.bodyEvents = new AbortController();
  }
  const store = visibilityStore();
  if (!applyingOrder) {
    body.querySelectorAll<HTMLElement>("[data-mail-nav-entry]").forEach((row) => {
      store.seed(SCOPE, visibilityKey(s.account, row.dataset.mailNavEntry!), row.dataset.visibility !== "hidden");
    });
  }
  renderCurrentVisibility(s.root);
  if (resetBaseline && !s.busy && !s.queued) s.baseline = s.root.outerHTML;
  if (!pageWindow.Sortable || body._appNavSortable) return;
  const dropIntoHeader = (event: MouseEvent): void => {
    const header = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-mail-nav-header]") : null;
    const dragged = pageWindow.Sortable?.dragged;
    if (
      !header || !dragged || dragged.parentElement !== body || s.settling || !dropAllowed(dragged, header, true, true)
    ) {
      return;
    }
    event.preventDefault();
    event.stopImmediatePropagation();
    dragIntoHeader(s, header, dragged);
  };
  body.addEventListener("dragenter", dropIntoHeader, { capture: true, signal: s.bodyEvents.signal });
  body.addEventListener("dragover", dropIntoHeader, { capture: true, signal: s.bodyEvents.signal });
  body._appNavSortable = new pageWindow.Sortable(body, {
    animation: 150,
    draggable: ROW,
    invertSwap: true,
    ghostClass: "opacity-30",
    handle: "[data-mail-nav-handle]",
    onMove: (event, original) => {
      if (s.settling) return false;
      if (isHeader(event.related)) {
        const bounds = event.related.getBoundingClientRect();
        if (
          original.clientY >= bounds.top && original.clientY <= bounds.bottom
          && dropAllowed(event.dragged, event.related, true, true)
        ) {
          dragIntoHeader(s, event.related, event.dragged);
        } else {
          clearHover(s);
        }
        return false;
      }
      clearHover(s);
      return dropAllowed(event.dragged, event.related, event.willInsertAfter);
    },
    onEnd: (event) => {
      clearHover(s);
      if (event.oldIndex !== event.newIndex) changed(s, event.item);
    },
  });
  body._appNavSortable.multiDrag?.destroyGlobal();
}
function changed(s: State, row: HTMLElement): void {
  s.moved = row.dataset.mailNavEntry ?? "";
  s.queued = true;
  const section = sectionOf(row);
  const next = row.nextElementSibling;
  s.moves.push({ key: s.moved, section, before: next instanceof HTMLElement ? next.dataset.mailNavEntry ?? "" : "" });
  const header = s.body.querySelector<HTMLElement>('[data-mail-nav-header][data-section="' + section + '"]');
  if (header) setSectionExpanded(header, true);
  for (const entry of rows(s)) {
    if (!isHeader(entry)) {
      entry.querySelector("td")?.setAttribute(
        "headers",
        s.root.id + "-name " + s.root.id + "-section-" + sectionOf(entry),
      );
    }
  }
  void visibilityStore().run(SCOPE, () => flush(s));
}
// The serial write lane owns submitted changes; removing a view ends its DOM
// subscriptions, so queued writes must drain after the root is disposed.
async function flush(s: State): Promise<void> {
  if (s.busy || !s.queued) return;
  s.busy = true;
  if (s.root.isConnected && !s.subscription.signal.aborted) {
    s.root.querySelectorAll<HTMLButtonElement>(
      'form[action="/settings/mail/nav"]:has(input[name="op"][value="reset"]) button',
    ).forEach((button) => button.disabled = true);
  }
  let failed = "";
  while (s.queued) {
    s.queued = false;
    const resetting = s.reset;
    s.reset = false;
    if (resetting) {
      s.settling = true;
    }
    const moves: Wire.NavMovesIn = s.moves.splice(0);
    const data = new URLSearchParams({
      op: resetting ? "reset" : "order",
      account: s.account,
      moves: JSON.stringify(moves),
    });
    try {
      const response = await fetch("/settings/mail/nav", {
        method: "POST",
        credentials: "same-origin",
        headers: { "HX-Request": "true", "HX-Current-URL": location.href },
        body: data,
      });
      if (response.headers.get("HX-Refresh") === "true") {
        s.busy = false;
        if (s.root.isConnected && !s.subscription.signal.aborted) location.reload();
        return;
      }
      const text = await response.text();
      if (!response.ok) {
        failed = text || "Could not save the sidebar order. Try again.";
        s.queued = false;
        s.moves = [];
        break;
      }
      s.baseline = text;
    } catch {
      failed = "Could not save the sidebar order. Check your connection and try again.";
      s.queued = false;
      s.moves = [];
      break;
    }
  }
  if (!s.root.isConnected || s.subscription.signal.aborted) {
    s.busy = false;
    return;
  }
  if (!pageWindow.htmx) {
    location.reload();
    return;
  }
  s.settling = true;
  applyingOrder = true;
  const expanded = new Map(
    Array.from(s.body.querySelectorAll<HTMLElement>("[data-mail-nav-header]"))
      .map(
        header => [
          header.dataset.section,
          header.querySelector("[aria-expanded]")?.getAttribute("aria-expanded") === "true",
        ],
      ),
  );
  const container = s.root.parentElement;
  const content = document.createElement("template");
  content.innerHTML = s.baseline;
  content.content.querySelectorAll<HTMLElement>("[data-mail-nav-header]").forEach(header => {
    const open = expanded.get(header.dataset.section);
    if (open !== undefined) setSectionExpanded(header, open);
  });
  try {
    await pageWindow.htmx.swap({ sourceElement: s.root, target: s.root, swap: "outerMorph", text: content.innerHTML });
  } catch {
    failed = "The order was saved, but the table could not refresh. Reload the page.";
  } finally {
    s.busy = false;
    s.settling = false;
    s.root.querySelectorAll<HTMLButtonElement>(
      'form[action="/settings/mail/nav"]:has(input[name="op"][value="reset"]) button',
    ).forEach((button) => button.disabled = false);
    if (container?.isConnected === true) discoverComponents(container);
    reconcile(s);
    applyingOrder = false;
  }
  const state = Array.from(states.values()).find(state => state.account === s.account && state.root.isConnected);
  if (state) {
    if (failed) announce(state, failed, true);
  }
}
function resetRows(s: State): void {
  const entries = Array.from(s.body.querySelectorAll<HTMLElement>("[data-mail-nav-entry]"));
  const sections = new Map(entries.map(row => [row, sectionOf(row)]));
  for (const header of Array.from(s.body.querySelectorAll<HTMLElement>("[data-mail-nav-header]"))) {
    const section = header.dataset.section!;
    const builtins = entries.filter(row => row.dataset.defaultSection === section)
      .sort((a, b) => Number(a.dataset.defaultPosition) - Number(b.dataset.defaultPosition));
    const saved = entries.filter(row => !row.dataset.defaultSection && sections.get(row) === section);
    let previous = header;
    for (const row of [...builtins, ...saved]) {
      previous.after(row);
      row.hidden = header.querySelector("[aria-expanded]")?.getAttribute("aria-expanded") === "false";
      row.querySelector("td")?.setAttribute("headers", s.root.id + "-name " + s.root.id + "-section-" + section);
      previous = row;
    }
  }
}
function moveByKey(s: State, row: HTMLElement, up: boolean): boolean {
  const name = row.querySelector('[data-slot="mail-settings-entry-name"]')?.textContent ?? "Mailbox";
  const neighbour = up ? row.previousElementSibling : row.nextElementSibling;
  let before: Element | null = null;
  if (neighbour && !isHeader(neighbour)) {
    if (neighbour.hasAttribute("data-mail-nav-fixed")) {
      announce(s, name + " cannot move past that row.");
      return true;
    }
    before = up ? neighbour : neighbour.nextElementSibling;
  } else {
    const headers = rows(s).filter(isHeader);
    const at = headers.findIndex((header) => header.dataset.section === sectionOf(row));
    let target: HTMLElement | undefined;
    for (let i = at + (up ? -1 : 1); i >= 0 && i < headers.length; i += up ? -1 : 1) {
      if (allows(row, headers[i].dataset.section ?? "")) {
        target = headers[i];
        break;
      }
    }
    if (!target) {
      announce(s, name + " is already " + (up ? "first" : "last"));
      return true;
    }
    before = up ? headers[headers.indexOf(target) + 1] ?? null : target.nextElementSibling;
    if (before?.hasAttribute("data-mail-nav-fixed") === true) {
      announce(s, name + " cannot move past that row.");
      return true;
    }
  }
  const focused = document.activeElement;
  s.queued = true;
  s.body.insertBefore(row, before);
  if (focused instanceof HTMLElement || focused instanceof SVGElement) focused.focus({ preventScroll: true });
  changed(s, row);
  return true;
}

function registerKeys(s: State): () => void {
  return on(ROOT + " " + ROW, (event, row, target) => {
    if (!s.root.contains(row) || s.settling || target.closest('[popover="auto"]')) return false;
    if (!(row instanceof HTMLElement)) return false;
    const current = row;
    if (matchChord(REORDER_UP, event) || matchChord(REORDER_DOWN, event)) {
      return current.hasAttribute("data-mail-nav-draggable") && moveByKey(s, current, matchChord(REORDER_UP, event));
    }
    return false;
  }, { id: "mail-settings-nav-rows-" + s.root.id, label: "Reorder the sidebar rows", owner: s.root, typing: "never" });
}
document.addEventListener("submit", (event) => {
  const form = event.target;
  if (
    !(form instanceof HTMLFormElement) || !form.closest(ROOT) || form.getAttribute("action") !== "/settings/mail/nav"
  ) return;
  event.preventDefault();
  event.stopPropagation();
  if (form.querySelector<HTMLInputElement>('input[name="op"]')?.value === "reset") {
    const s = states.get(form.closest<HTMLElement>(ROOT)!);
    if (s) {
      resetRows(s);
      s.moves = [];
      s.reset = true;
      s.queued = true;
      s.settling = true;
      s.moved = "";
      void visibilityStore().run(SCOPE, () => flush(s));
    }
    return;
  }
  const entry = form.querySelector<HTMLInputElement>('input[name="entry"]')?.value;
  const account = form.closest<HTMLElement>(ROOT)?.dataset.account;
  if (!entry || !account) return;
  const key = visibilityKey(account, entry);
  const store = visibilityStore();
  store.set(SCOPE, key, !(store.get(SCOPE, key) ?? true));
}, { capture: true });
document.addEventListener("dragover", (event) => {
  for (const state of states.values()) {
    if (!(event.target instanceof Node) || state.hoverHeader?.contains(event.target) !== true) clearHover(state);
  }
}, { capture: true });
document.addEventListener("dragleave", (event) => {
  for (const state of states.values()) {
    if (!(event.relatedTarget instanceof Node) || state.hoverHeader?.contains(event.relatedTarget) !== true) {
      clearHover(state);
    }
  }
}, { capture: true });
document.addEventListener("dragend", () => states.forEach(clearHover), { capture: true });
document.addEventListener("htmx:after:swap", (event) => {
  const target = event.detail?.ctx?.target ?? event.target;
  if (!(target instanceof Element)) return;
  discoverComponents(target);
  for (const state of states.values()) {
    if (state.root.contains(target) || target.contains(state.root)) reconcile(state, true);
  }
});

registerComponent(ROOT, (root, signal) => {
  const s: State = {
    account: root.dataset.account!,
    moves: [],
    root,
    body: root.querySelector<SortableBody>("[data-mail-nav-sortable]") ?? root,
    baseline: root.outerHTML,
    busy: false,
    queued: false,
    reset: false,
    settling: false,
    moved: "",
    subscription: new AbortController(),
    bodyEvents: new AbortController(),
  };
  states.set(root, s);
  const unregisterKeys = registerKeys(s);
  root.addEventListener("click", event => {
    const toggle = event.target instanceof Element
      ? event.target.closest<HTMLElement>('[data-slot="mail-view-section-toggle"]')
      : null;
    const header = toggle?.closest<HTMLElement>("[data-mail-nav-header]");
    if (header?.closest(ROOT) === root) setSectionExpanded(header, toggle?.getAttribute("aria-expanded") !== "true");
  }, { signal });
  const observer = new MutationObserver(changes => {
    if (
      changes.some(change => {
        if (change.type === "attributes") return change.target instanceof Element && change.target.matches(ROW);
        return [...change.addedNodes, ...change.removedNodes].some(node =>
          node instanceof Element
          && (node.matches(ROW + ", [data-mail-nav-sortable], " + CONTROL)
            || node.querySelector(ROW + ", " + CONTROL) !== null)
        );
      })
    ) reconcile(s);
  });
  observer.observe(root, {
    attributeFilter: ["hidden", "data-visibility"],
    childList: true,
    subtree: true,
  });
  signal.addEventListener("abort", () => {
    observer.disconnect();
    disposeBody(s);
    s.subscription.abort();
    unregisterKeys();
    states.delete(root);
  }, { once: true });
  reconcile(s);
  visibilityStore().subscribe(
    SCOPE,
    null,
    (key, visible) => renderVisibility(key, visible, root),
    s.subscription.signal,
  );
});
