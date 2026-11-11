import * as Arr from "effect/Array";
import * as Str from "effect/String";

import { htmxWrite } from "./htmx_write";
import { installLeaveFlush } from "./intents";
import {
  chipShown,
  createLabelIntents,
  editValue,
  type LabelFacts,
  type LabelHost,
  type LabelIntents,
  type LabelProjection,
  REMOVED,
} from "./labelintents";
import { registerComponent } from "./lifecycle";
import { toastAPI } from "./template_scripts/toast";

// The sidebar's label menu (uimail.labelMenu) and the label dialogs it
// opens, drawn from the label intents (labelintents.ts): a visibility
// choice, an edit and a removal show in the sidebar and on mail rows at
// once and are saved after. Each label's ⋮ carries the label as the server
// drew it and is never written to, so it is always what the server
// confirmed; everything drawn from the intents is drawn again over a sidebar
// or mail rows the server swaps in.

const SIDEBAR = "#mail-sidebar-content";
const OPENER = 'button[data-menu-opener="label-menu"][data-label-id]';
const CHIP = '[data-slot="mail-row-labels"] [data-label-id]';
// The event a label write's answer raises when Gmail stopped partway
// (uimail.LabelWritesStoppedEvent).
const WRITES_STOPPED = "mail:label-writes-stopped";

// raised says whether an answer's HX-Trigger names the event, as htmx reads
// a list of names.
function raised(headers: Headers, event: string): boolean {
  return Arr.some(Str.split(headers.get("HX-Trigger") ?? "", ","), (name) => Str.trim(name) === event);
}

// sidebars are the account's sidebar lists that draw labels.
// formText is a form field's text, or "" when the form has none: a label
// form has no file field.
function formText(data: FormData, name: string): string {
  const value = data.get(name);
  return typeof value === "string" ? value : "";
}

function sidebars(account?: string): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>(SIDEBAR + " [data-count-account]"))
    .filter((root) => account === undefined || root.dataset.countAccount === account);
}

function accountOf(element: Element): string | undefined {
  return element.closest<HTMLElement>("[data-count-account]")?.dataset.countAccount;
}

function openers(root: ParentNode): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(OPENER));
}

function factsOf(opener: HTMLElement): LabelFacts {
  const data = opener.dataset;
  const facts: LabelFacts = {
    housecat: data.labelEditable === "false",
    id: data.labelId!,
    list: data.labelList ?? "",
    message: data.labelMessage ?? "",
    name: data.labelName ?? "",
    parent: data.labelParent ?? "",
  };
  if (data.labelUnread !== undefined) facts.unread = Number(data.labelUnread);
  return facts;
}

function setText(element: Element | null | undefined, text: string): void {
  if (element && element.textContent !== text) element.textContent = text;
}

function setAttr(element: Element | null | undefined, name: string, value: string): void {
  if (element && element.getAttribute(name) !== value) element.setAttribute(name, value);
}

function setHidden(element: HTMLElement | null | undefined, hidden: boolean): void {
  if (element && element.hidden !== hidden) element.hidden = hidden;
}

// arrange puts items in ul in order, contiguous and before anchor (null for
// the end), moving only those out of place.
function arrange(ul: Element, items: HTMLElement[], anchor: Element | null): void {
  let next = anchor;
  for (let i = items.length - 1; i >= 0; i--) {
    const item = items[i];
    if (item.parentElement !== ul || item.nextElementSibling !== next) ul.insertBefore(item, next);
    next = item;
  }
}

function folderOf(li: Element, id: string): HTMLElement | null {
  return li.querySelector<HTMLElement>(`:scope > details[data-nav-section="L:${CSS.escape(id)}"]`);
}

// drawSidebar moves and renames the label rows of one sidebar list as the
// projection has them.
function drawSidebar(root: HTMLElement, projection: LabelProjection): void {
  const section = root.querySelector<HTMLElement>('details[data-nav-section="labels"] > ul');
  const more = section?.querySelector<HTMLElement>(':scope > li > details[data-nav-section="labels-more"]');
  const moreList = more?.querySelector<HTMLElement>(":scope > ul");
  if (!section || !more || !moreList) return;
  const rows = new Map<string, HTMLElement>();
  for (const opener of openers(root)) {
    const li = opener.closest("li");
    if (li) rows.set(opener.dataset.labelId!, li);
  }
  const rowsOf = (ids: string[]) => ids.flatMap((id) => rows.get(id) ?? []);
  for (const [id, place] of projection.labels) {
    const li = rows.get(id);
    if (!li) continue;
    const row = li.querySelector(':scope > [data-slot="tooltip"]');
    setText(row?.querySelector('[data-slot="nav-item-label"]'), place.own);
    setText(row?.querySelector('[data-slot="tooltip-popup"]'), place.name);
    setAttr(row?.querySelector(OPENER), "aria-label", "Options for " + place.name);
    setAttr(row?.querySelector(OPENER), "data-menu-title", place.name.replaceAll("/", " / "));
    if (li.style.getPropertyValue("--nav-row-depth") !== String(place.depth)) {
      li.style.setProperty("--nav-row-depth", String(place.depth));
    }
    setHidden(li, place.removed);
    setAttr(folderOf(li, id)?.querySelector(":scope > summary"), "aria-label", "Sublabels of " + place.name);
  }
  const template = root.querySelector<HTMLTemplateElement>('template[data-slot="label-folder-template"]');
  for (const [parent, kids] of projection.children) {
    const li = rows.get(parent);
    if (!li) continue;
    let folder = folderOf(li, parent);
    if (!folder && template?.content.firstElementChild) {
      const copy = template.content.firstElementChild.cloneNode(true);
      if (copy instanceof HTMLDetailsElement) {
        copy.dataset.navSection = "L:" + parent;
        copy.querySelector(":scope > summary")?.setAttribute(
          "aria-label",
          "Sublabels of " + projection.labels.get(parent)!.name,
        );
        copy.open = true;
        li.append(copy);
        folder = copy;
      }
    }
    const list = folder?.querySelector(":scope > ul");
    if (list) arrange(list, rowsOf(kids), null);
  }
  arrange(section, rowsOf(projection.shown), more.parentElement);
  arrange(moreList, rowsOf(projection.more), null);
  setHidden(more.parentElement, projection.more.every((id) => projection.labels.get(id)!.removed));
  for (const [id, li] of rows) {
    const folder = folderOf(li, id);
    if (folder && !folder.querySelector(":scope > ul > li")) folder.remove();
  }
}

// drawChips hides the chips of labels hidden from the message list or
// removed, shows again those it hid, and renames the rest.
function drawChips(projection: LabelProjection): void {
  document.querySelectorAll<HTMLElement>(CHIP).forEach((chip) => {
    const place = projection.labels.get(chip.dataset.labelId!);
    if (!place) return;
    const hide = !chipShown(place);
    if (hide && chip.hidden === false) {
      chip.hidden = true;
      chip.dataset.labelMenuHidden = "";
    } else if (!hide && chip.dataset.labelMenuHidden !== undefined) {
      chip.hidden = false;
      delete chip.dataset.labelMenuHidden;
    }
    const name = chip.querySelector<HTMLElement>("[data-label-name]");
    if (name && name.dataset.labelName !== place.name) {
      name.dataset.labelName = place.name;
      setText(name.firstElementChild ?? name, place.name);
    }
  });
}

const observers = new Map<HTMLElement, MutationObserver>();

const host: LabelHost = {
  facts: (account) => sidebars(account).slice(0, 1).flatMap((root) => openers(root).map(factsOf)),
  draw(account, projection) {
    for (const root of sidebars(account)) drawSidebar(root, projection);
    drawChips(projection);
    // What was drawn here is not the server's: the sidebars' observers
    // need not draw it again.
    observers.forEach((observer) => observer.takeRecords());
  },
  async post(url, body, keepalive) {
    const answer = await htmxWrite(url, body, keepalive);
    return { ok: answer.ok, stopped: raised(answer.headers, WRITES_STOPPED), text: answer.text };
  },
  reloadRows: () => window.HousecatContent.reload(),
  toast: (message) => toastAPI.show({ message }),
};

const intents: LabelIntents = createLabelIntents(host);
installLeaveFlush(intents.store);

// seedAll reads every sidebar the server drew into the intents and draws
// what is wanted over it.
function seedAll(root: ParentNode = document): void {
  const accounts = new Set(
    Array.from(root.querySelectorAll<HTMLElement>("[data-count-account]")).map((el) => el.dataset.countAccount!),
  );
  accounts.forEach((account) => intents.seed(account));
}

registerComponent(SIDEBAR, (root, signal) => {
  // The server drawing the sidebar again -- a morph after a change, a count
  // or a section -- is seeded and drawn over.
  const observer = new MutationObserver(() => seedAll(root));
  observer.observe(root, {
    attributeFilter: [
      "data-label-list",
      "data-label-message",
      "data-label-name",
      "data-label-parent",
      "data-label-unread",
    ],
    childList: true,
    subtree: true,
  });
  observers.set(root, observer);
  signal.addEventListener("abort", () => {
    observer.disconnect();
    observers.delete(root);
  }, { once: true });
  seedAll(root);
});

// Mail rows the server swaps in are drawn over too.
document.addEventListener("htmx:after:swap", () => {
  sidebars().forEach((root) => intents.draw(root.dataset.countAccount!));
});

// Before the label menu opens from a label's ⋮, it takes the label: the
// label it saves for, the choice ticked in each group as it is wanted, no
// "In label list" group for a sublabel (Gmail applies that setting to
// top-level labels only, so the app does not offer it), and, for a label the
// app may only show or hide, no Edit, Remove label or Add sublabel. Nothing
// is fetched.
document.addEventListener("menu:before-open", (event) => {
  const opener = event.detail.opener;
  const menu = event.target;
  if (!opener.hasAttribute("data-label-id") || !(menu instanceof HTMLElement)) return;
  const account = accountOf(opener);
  const place = account === undefined ? undefined : intents.projection(account).labels.get(opener.dataset.labelId!);
  if (!place) return;
  const field = menu.querySelector<HTMLInputElement>('input[name="label"]');
  if (field) field.value = place.id;
  menu.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]').forEach((item) => {
    const current = item.name === "list" ? place.list : place.message;
    setAttr(item, "aria-checked", String(item.value === current));
  });
  menu.querySelectorAll<HTMLElement>("[data-top-level-only]").forEach((part) => setHidden(part, place.depth > 0));
  menu.querySelectorAll<HTMLElement>("[data-editable-only]").forEach((part) => setHidden(part, place.housecat));
});

function refuse(event: Event): void {
  event.preventDefault();
  event.stopPropagation();
}

// normalizedName is a typed name as the server stores it: each level
// trimmed; empty when a level is empty.
function normalizedName(typed: string): string {
  const levels = typed.split("/").map((level) => level.trim());
  return levels.some((level) => !level) ? "" : levels.join("/");
}

// under says whether the label is id or one of its sublabels as drawn.
function under(projection: LabelProjection, label: string, id: string): boolean {
  for (let at = label, steps = 0; at && steps <= projection.labels.size; steps++) {
    if (at === id) return true;
    at = projection.labels.get(at)?.parent ?? "";
  }
  return false;
}

// accountOfForm is the mailbox a dialog's form saves to, which the server
// put in its post URL (uimail.LabelURL).
function accountOfForm(form: HTMLFormElement): string {
  return new URL(form.getAttribute("hx-post") ?? "", location.href).searchParams.get("account") ?? "";
}

// A visibility choice, by click or key (the menu clicks an item for Enter
// and Space), and an Edit or Remove label dialog's submit are saved through
// the intents instead of being posted. The edit is checked here where that
// is cheap -- the server checks everything again.
document.addEventListener("submit", (event) => {
  const form = event.target;
  if (!(form instanceof HTMLFormElement)) return;
  if (form.hasAttribute("data-sidebar-label-menu")) {
    refuse(event);
    const choice = event.submitter;
    const id = form.querySelector<HTMLInputElement>('input[name="label"]')?.value;
    const account = accountOf(form);
    if (!(choice instanceof HTMLButtonElement) || !id || account === undefined) return;
    if (choice.name !== "list" && choice.name !== "message") return;
    form.querySelectorAll<HTMLButtonElement>(`[role="menuitemradio"][name="${choice.name}"]`).forEach((item) => {
      setAttr(item, "aria-checked", String(item === choice));
    });
    intents.set(account, choice.name, id, choice.value);
    return;
  }
  const intent = form.dataset.labelIntent;
  if (intent !== "edit" && intent !== "remove") return;
  refuse(event);
  const id = form.querySelector<HTMLInputElement>('input[name="label"]')?.value;
  if (!id) return;
  const account = accountOfForm(form);
  if (intent === "remove") {
    form.closest("dialog")?.close();
    intents.set(account, "remove", id, REMOVED);
    return;
  }
  const data = new FormData(form);
  const name = normalizedName(formText(data, "name"));
  const nested = data.get("nest");
  const nest = nested !== null && nested !== "";
  const parent = nest ? formText(data, "parent") : "";
  const projection = intents.projection(account);
  let refusal = "";
  if (!name) refusal = "Enter a name with no empty level.";
  else if (nest && !parent) refusal = "Choose the label to nest it under.";
  else if (parent && under(projection, parent, id)) refusal = "A label can't be nested under itself or its sublabels.";
  if (refusal) {
    host.toast(refusal);
    return;
  }
  form.closest("dialog")?.close();
  intents.set(account, "edit", id, editValue(name, parent));
}, { capture: true });
