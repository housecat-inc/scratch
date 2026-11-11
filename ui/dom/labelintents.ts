import * as Predicate from "effect/Predicate";

import type * as Wire from "../schemas/schemas.gen";
import { createIntentStore, type IntentRuntime, type IntentStore } from "./intents";
import { messageOf } from "./request_outcome";

// Label intents are what the user asked of the sidebar's labels from the
// label menu -- where a label shows, its new name and parent, its removal --
// held in one intent store per account until Gmail has them. The sidebar is
// drawn from them at once (projectLabels); the save follows, folded into as
// few requests as the endpoints take, one batch at a time per account, so a
// change to a label made while an earlier one is in flight goes after it.
//
// This file knows no DOM: labelmenu.ts reads what the server drew into
// LabelFacts and draws a LabelProjection, so the rules here run in Node.

// A key is a field and a label ID, field:id.
export type LabelField = "edit" | "list" | "message" | "remove";

// Gmail's labelListVisibility and messageListVisibility values, and the
// value a removal is wanted with.
export const LIST_HIDE = "labelHide";
export const LIST_SHOW = "labelShow";
export const LIST_SHOW_IF_UNREAD = "labelShowIfUnread";
export const MESSAGE_HIDE = "hide";
export const MESSAGE_SHOW = "show";
export const REMOVED = "removed";

// LabelFacts is a label as the server last drew it.
export type LabelFacts = {
  // Housecat's own (🐈) labels: listed first, and only shown or hidden.
  housecat: boolean;
  id: string;
  list: string;
  message: string;
  // The full name, nesting included.
  name: string;
  // The label it is drawn under; empty at the top.
  parent: string;
  // The unread count, where the sync has read one.
  unread?: number;
};

// LabelPlace is a label as it is to be drawn.
export type LabelPlace = {
  depth: number;
  housecat: boolean;
  id: string;
  list: string;
  message: string;
  name: string;
  // The part of its name drawn under its parent.
  own: string;
  parent: string;
  // Removed, or under a removed label.
  removed: boolean;
};

// LabelProjection is the sidebar as it is to be drawn: every label, and each
// list's labels in order -- shown at the top, under More, and under each
// parent (children, by parent ID).
export type LabelProjection = {
  children: Map<string, string[]>;
  labels: Map<string, LabelPlace>;
  more: string[];
  shown: string[];
};

export function labelKey(field: LabelField, id: string): string {
  return field + ":" + id;
}

function splitKey(key: string): [string, string] {
  const at = key.indexOf(":");
  return [key.slice(0, at), key.slice(at + 1)];
}

// editValue is an edit's value: the label's own name and the label it nests
// under, as the Edit label dialog posts them.
export function editValue(own: string, parent: string): string {
  return JSON.stringify([own, parent]);
}

function parseEdit(value: string): [string, string] {
  const edit: unknown = JSON.parse(value);
  if (!Array.isArray(edit) || edit.length !== 2 || !edit.every(Predicate.isString)) {
    throw new TypeError("label edit is not [name, parent]: " + value);
  }
  return [edit[0], edit[1]];
}

// ownName is the part of facts' name drawn under its parent.
function ownName(facts: LabelFacts, base: Map<string, LabelFacts>): string {
  const parent = base.get(facts.parent);
  return parent && facts.name.startsWith(parent.name + "/") ? facts.name.slice(parent.name.length + 1) : facts.name;
}

// compareLabelNames orders labels as Gmail lists them, as
// uimail.CompareLabelNames does: ignoring case, then exactly.
export function compareLabelNames(a: string, b: string): number {
  const la = a.toLowerCase();
  const lb = b.toLowerCase();
  if (la !== lb) return la < lb ? -1 : 1;
  return a === b ? 0 : a < b ? -1 : 1;
}

// listHidden says whether a top-level label goes under More, as the server's
// labelListHidden does: hidden, or shown only while unread with none unread.
// An unknown count leaves it shown.
function listHidden(list: string, unread: number | undefined): boolean {
  return list === LIST_HIDE || (list === LIST_SHOW_IF_UNREAD && unread === 0);
}

// seedValues are the store's confirmed values for facts.
export function seedValues(facts: LabelFacts[]): [string, string][] {
  const base = new Map(facts.map((f) => [f.id, f]));
  return facts.flatMap((f) => [
    [labelKey("list", f.id), f.list],
    [labelKey("message", f.id), f.message],
    [labelKey("edit", f.id), editValue(ownName(f, base), f.parent)],
    [labelKey("remove", f.id), ""],
  ]);
}

// projectLabels lays what the user wants (get, by key) over facts: names
// and parents from edits, so a renamed label's sublabels are renamed with
// it; removals, which take the subtree; and each list in Gmail's order --
// 🐈 labels first at the top and under More, then by name. Only a top-level
// label's list visibility moves it, as in Gmail.
export function projectLabels(facts: LabelFacts[], get: (key: string) => string | undefined): LabelProjection {
  const base = new Map(facts.map((f) => [f.id, f]));
  const edits = new Map<string, [string, string]>();
  for (const f of facts) {
    const wanted = get(labelKey("edit", f.id));
    edits.set(f.id, wanted === undefined ? [ownName(f, base), f.parent] : parseEdit(wanted));
  }
  // A parent that is gone, or a loop, puts the label at the top.
  const parentOf = (id: string): string => {
    const parent = edits.get(id)![1];
    if (!base.has(parent)) return "";
    for (let at = parent, steps = 0; at; at = edits.get(at)?.[1] ?? "", steps++) {
      if (at === id || steps > base.size) return "";
    }
    return parent;
  };
  const labels = new Map<string, LabelPlace>();
  const place = (id: string): LabelPlace => {
    const known = labels.get(id);
    if (known) return known;
    const f = base.get(id)!;
    const own = edits.get(id)![0];
    const parentID = parentOf(id);
    const parent = parentID ? place(parentID) : undefined;
    const out: LabelPlace = {
      depth: parent ? parent.depth + 1 : 0,
      housecat: f.housecat,
      id,
      list: get(labelKey("list", id)) ?? f.list,
      message: get(labelKey("message", id)) ?? f.message,
      name: parent ? parent.name + "/" + own : own,
      own,
      parent: parentID,
      removed: get(labelKey("remove", id)) === REMOVED || parent?.removed === true,
    };
    labels.set(id, out);
    return out;
  };
  facts.forEach((f) => place(f.id));
  const shown: string[] = [];
  const more: string[] = [];
  const children = new Map<string, string[]>();
  for (const p of labels.values()) {
    if (p.parent) {
      const kids = children.get(p.parent) ?? [];
      kids.push(p.id);
      children.set(p.parent, kids);
    } else if (listHidden(p.list, base.get(p.id)!.unread)) {
      more.push(p.id);
    } else {
      shown.push(p.id);
    }
  }
  const byName = (a: string, b: string) => compareLabelNames(labels.get(a)!.name, labels.get(b)!.name);
  const top = (a: string, b: string) =>
    Number(labels.get(b)!.housecat) - Number(labels.get(a)!.housecat) || byName(a, b);
  shown.sort(top);
  more.sort(top);
  children.forEach((kids) => kids.sort(byName));
  return { children, labels, more, shown };
}

// chipShown says whether mail rows draw the label's chip: not when it is
// hidden from the message list or removed.
export function chipShown(place: LabelPlace): boolean {
  return !place.removed && place.message !== MESSAGE_HIDE;
}

// LabelResponse is what a post answered: whether it succeeded; whether Gmail
// stopped partway, which the answer's toast says, leaving the rest of the
// turn unsent; and the body, an error's message in its JSON.
export type LabelResponse = { ok: boolean; stopped: boolean; text: string };

// LabelHost is the page the intents draw and save through.
export interface LabelHost {
  // The labels of the account's sidebar as the server drew them.
  facts: (account: string) => LabelFacts[];
  // Draw the projection.
  draw: (account: string, projection: LabelProjection) => void;
  // Post a form, keepalive for a page being left. It resolves once the
  // sidebar a successful answer draws, and its toast, are swapped in.
  post: (url: string, body: URLSearchParams, keepalive: boolean) => Promise<LabelResponse>;
  // Draw the mail rows again, for a change they show.
  reloadRows: () => void;
  toast: (message: string) => void;
}

export const RETRY = "Couldn't save the label change. Try again.";

type Change = { key: string; value: string };
type Result = { error: string; rows: boolean };

// labelURL is the label endpoint for action in the account, as
// uimail.LabelURL builds it.
export function labelURL(action: string, account: string): string {
  const path = "/mail/labels/" + action;
  return account ? path + "?" + new URLSearchParams({ account }).toString() : path;
}

// requests folds an account's pending changes into what the endpoints take:
// every visibility in one batch, then each edit, then each removal, so a
// label is changed before it is removed.
function requests(account: string, changes: Change[]): { body: URLSearchParams; url: string }[] {
  const visibility: NonNullable<Wire.LabelVisibilityChangesIn>[number][] = [];
  const edits: { body: URLSearchParams; url: string }[] = [];
  const removals: { body: URLSearchParams; url: string }[] = [];
  for (const { key, value } of changes) {
    const [field, label] = splitKey(key);
    if (field === "list" || field === "message") visibility.push({ field, label, value });
    if (field === "edit") {
      const [name, parent] = parseEdit(value);
      const body = new URLSearchParams({ label, name });
      if (parent) {
        body.set("nest", "on");
        body.set("parent", parent);
      }
      edits.push({ body, url: labelURL("edit", account) });
    }
    if (field === "remove" && value === REMOVED) {
      removals.push({ body: new URLSearchParams({ label }), url: labelURL("remove", account) });
    }
  }
  const out = visibility.length
    ? [{ body: new URLSearchParams({ changes: JSON.stringify(visibility) }), url: labelURL("visibility", account) }]
    : [];
  return [...out, ...edits, ...removals];
}

export interface LabelIntents {
  readonly store: IntentStore<string>;
  // Read what the server drew for the account into the store and draw.
  seed: (account: string) => void;
  // Draw the account's sidebar from what is wanted.
  draw: (account: string) => void;
  // The account's labels as they are to be drawn.
  projection: (account: string) => LabelProjection;
  // Ask for a change; it is drawn at once and saved after.
  set: (account: string, field: LabelField, id: string, value: string) => void;
}

// createLabelIntents keeps the label intents for host's sidebars. A save
// Gmail turns down before anything is changed puts back what the server
// has and says why. One it turns down partway keeps what was done -- the
// sidebar the answer drew shows it, and its toast says what was not; once
// the account has nothing in flight, the store is seeded again from that
// sidebar, so it holds what the server has.
export function createLabelIntents(host: LabelHost, runtime?: IntentRuntime): LabelIntents {
  const failures = new Map<string, string>();
  const project = (account: string) => projectLabels(host.facts(account), (key) => intents.store.get(account, key));
  const reseed = (account: string) => {
    for (const [key, value] of seedValues(host.facts(account))) intents.store.seed(account, key, value);
    host.draw(account, project(account));
  };
  const store = createIntentStore<string, Change[], Result>({
    fold: (changes) => changes.length ? changes : null,
    async send(account, changes, keepalive) {
      const rows = changes.some((c) => !c.key.startsWith("list:"));
      let error = "";
      const queue = requests(account, changes);
      for (const [i, request] of queue.entries()) {
        let response: LabelResponse;
        try {
          response = await host.post(request.url, request.body, keepalive);
        } catch {
          if (i === 0) {
            failures.set(account, RETRY);
            throw new Error(RETRY);
          }
          error = RETRY;
          break;
        }
        if (!response.ok) {
          const message = messageOf(response.text) || RETRY;
          if (i === 0) {
            failures.set(account, message);
            throw new Error(message);
          }
          error = message;
          break;
        }
        if (response.stopped) break;
      }
      return { error, rows };
    },
    answered(account, result) {
      host.draw(account, project(account));
      if (!result) return;
      if (result.error) host.toast(result.error);
      if (result.rows) host.reloadRows();
      void intents.store.settled(account).then(() => reseed(account));
    },
    failed(account) {
      host.draw(account, project(account));
      host.toast(failures.get(account) ?? RETRY);
      failures.delete(account);
    },
  }, runtime);
  const intents: LabelIntents = {
    store,
    seed: reseed,
    draw: (account) => host.draw(account, project(account)),
    projection: project,
    set(account, field, id, value) {
      store.set(account, labelKey(field, id), value);
      host.draw(account, project(account));
    },
  };
  return intents;
}
