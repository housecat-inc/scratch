// Modal layers and the Escape stack, for the router in keys.ts.
//
// A layer is read from the DOM on every key and every mutation batch -- the
// elements a selector matches, or a predicate -- so nothing opens or closes
// one: a drawer rendered open, or one an htmx swap replaced, is right on the
// next key. Of the active layers one is on top: a layer whose regions lie
// inside another's goes above it, else the one first seen open latest, and
// a layer may name layers it always stays above -- by a list, or by a
// function read from the element it matched, so each of one layer's elements
// can name its own. Keys never travel past the top layer's regions, which may
// be disjoint, as the tour's anchor and its step popover are.
// What is inert under the stack follows the topmost layer that asks to be
// modal, with the layers stacked over it, rather than the top layer alone.
//
// The Escape stack is what one Escape closes before the top layer's own
// Escape runs: the newest open auto popover, stamped as it opens, or
// dismissable a script declared with scope().

import { safely } from "./keyreport";

export type LayerOptions = {
  active?: () => boolean;
  back?: boolean;
  escape: (event: KeyboardEvent, el?: Element) => boolean | void;
  id: string;
  identity?: string;
  initial?: (el?: Element) => HTMLElement | null;
  match?: string;
  media?: string;
  modal?: boolean | ((el?: Element) => boolean);
  over?: string[] | ((el?: Element) => string[]);
  own?: (el?: Element) => Element[];
  owner?: Element;
  regions: (el?: Element) => (Element | null)[];
};

export type ScopeOptions = {
  close: () => void;
  element: Element;
};

// ScopeHandle is what a script holds for its scope: close takes the entry off
// the Escape stack, and suspend and resume take it out of the Escape order and
// put it back, keeping its stamp.
export type ScopeHandle = { close: () => void; resume: () => void; suspend: () => void };

type Def = {
  o: LayerOptions;
  owner: Element | null;
};

// Instance is a layer active now: el is the element its match selected, key
// identifies it from one key or mutation batch to the next, over is the layer
// ids it always stays above, read for el, and stamp orders it by when it was
// first seen active.
export type Instance = {
  def: Def;
  el: Element | null;
  key: object;
  over: string[];
  own: Element[];
  regions: Element[];
  stamp: number;
};

// A Dismissable's stamp never changes once taken, including across a scope's
// suspend and resume: suspended is read by newestDismissable, which skips it
// while true, so a scope a script suspends and resumes keeps its place in the
// Escape order rather than rising above entries opened while it was away.
export type Dismissable = {
  close: () => void;
  element: Element;
  invoker: Element | null;
  stamp: number;
  suspended?: boolean;
};

// open and data-open are how dialogs, details, the tour's overlay and the
// palette say they are open; aria-hidden and inert are watched so a morph
// that strips the router's marks gets them back.
const BASE_ATTRIBUTES = ["aria-hidden", "data-open", "inert", "open"];

// Stamp is when a layer was first seen active, under the id it registered
// with, and, for an element key, where that element last sat while connected
// -- its parent and index among the parent's children -- so a later swap can
// tell whether a new element plausibly replaced it.
type Stamp = {
  at: number;
  id: string;
  parent: Element | null;
  stamp: number;
};

const defs: Def[] = [];
const stamps = new Map<object, Stamp>();
const popovers = new Map<Element, Dismissable>();
const scopes = new Set<Dismissable>();
let clock = 0;

// addLayer registers a layer, replacing one with the same id. A layer
// registered again, as a script that runs again does, keeps its place in the
// stack.
export function addLayer(o: LayerOptions, owner: Element | null): void {
  const def: Def = { o, owner };
  const at = defs.findIndex((old) => old.o.id === o.id);
  if (at >= 0) {
    const old = defs[at];
    defs.splice(at, 1);
    const stamp = stamps.get(old);
    if (stamp) {
      stamps.delete(old);
      stamps.set(def, stamp);
    }
  }
  defs.push(def);
}

// watchedAttributes is every attribute a layer's match selector reads, which
// the router's mutation observer has to see change.
export function watchedAttributes(): string[] {
  const names = new Set(BASE_ATTRIBUTES);
  for (const def of defs) {
    for (const found of (def.o.match ?? "").matchAll(/\[\s*([\w-]+)/g)) names.add(found[1]);
  }
  return Array.from(names).sort();
}

// modal is whether top asks for the page outside its regions to be inert. A
// predicate is read on every projection, so a record on the layer's element
// decides, and the attribute it reads has to change in the same task as one
// the router watches, or the script has to settle the router itself.
export function modal(top: Instance): boolean {
  // A script's predicate may be untyped JS, so only true counts.
  const o: boolean | ((el?: Element) => unknown) | undefined = top.def.o.modal;
  if (typeof o === "function") return safely(() => o(top.el ?? undefined) === true, false);
  return o === true;
}

// connected lists the elements a layer's regions or own gave that are in the
// document. A script may give an array, a NodeList or one element.
function connected(nodes: unknown): Element[] {
  const list: unknown[] = nodes instanceof Element
    ? [nodes]
    : Array.isArray(nodes) || nodes instanceof NodeList || nodes instanceof HTMLCollection
    ? Array.from<unknown>(nodes)
    : [];
  return list.filter((node): node is Element => node instanceof Element && node.isConnected);
}

// siblingIndex is where el sits among its parent's children, or -1 outside
// one. Read while el is still connected, since a removed element's parent is
// gone.
function siblingIndex(el: Element): number {
  const parent = el.parentElement;
  return parent ? Array.prototype.indexOf.call(parent.children, el) : -1;
}

// plausible is whether key plausibly replaced was, the disconnected element
// stamp was last taken from. When both carry the layer's identity attribute
// -- id, unless the layer names another -- it decides: a swap keeps an
// element's identity wherever it puts the element, even under a new parent or
// at a new index, and two different values are two different elements.
// Otherwise the same parent at the same index decides it, which a same-place
// replacement keeps even with no identity. Anything else -- an unrelated
// element that merely shares was's layer id, as two dialog instances do -- takes a
// fresh stamp instead of was's.
function plausible(key: Element, was: Element, stamp: Stamp, identity: string): boolean {
  const now = key.getAttribute(identity) ?? "";
  const then = was.getAttribute(identity) ?? "";
  if (now !== "" && then !== "") return now === then;
  return stamp.parent !== null && stamp.parent === key.parentElement && stamp.at === siblingIndex(key);
}

// stampOf is when the layer keyed by key was first seen active. An element
// that plausibly replaced a removed one of the same layer id, as a swap does,
// takes over the removed one's stamp, so the swap never lifts an older layer
// above a newer one; an unrelated element new to the same layer id takes a
// fresh stamp instead, so it never inherits a stale one's place by accident.
function stampOf(key: object, id: string, identity: string): number {
  const place = key instanceof Element
    ? { at: siblingIndex(key), parent: key.parentElement }
    : { at: -1, parent: null };
  const known = stamps.get(key);
  if (known) return Object.assign(known, place).stamp;
  if (key instanceof Element) {
    for (const [was, stamp] of Array.from(stamps)) {
      if (stamp.id !== id || !(was instanceof Element) || was.isConnected) continue;
      if (!plausible(key, was, stamp, identity)) continue;
      stamps.delete(was);
      stamps.set(key, Object.assign(stamp, place));
      return stamp.stamp;
    }
  }
  const stamp: Stamp = { id, stamp: ++clock, ...place };
  stamps.set(key, stamp);
  return stamp.stamp;
}

// active lists the layers active now: open, by their match or active(), and
// with their media matching. A layer is stamped the first time it is seen
// open and forgotten once it is not, so one that closes and opens again is
// newer than one that stayed open. A media query that stops matching
// suspends an open layer rather than closing it: the layer is left out of
// the list but keeps its stamp, so a breakpoint crossing and back puts it
// where it was, whatever order the layers were registered in meanwhile.
export function active(): Instance[] {
  for (let i = defs.length - 1; i >= 0; i--) {
    const owner = defs[i].owner;
    if (owner && !owner.isConnected) defs.splice(i, 1);
  }
  const out: Instance[] = [];
  const open = new Set<object>();
  for (const def of defs) {
    const { o } = def;
    const match = o.match;
    // A script's active() may be untyped JS, so only true counts.
    const els: (Element | null)[] = match
      ? safely(() => Array.from(document.querySelectorAll(match)), [])
      : safely(() => !o.active || (o.active as () => unknown)() === true, false)
      ? [null]
      : [];
    if (!els.length) continue;
    const media = o.media;
    const shown = !media || safely(() => matchMedia(media).matches, false);
    for (const el of els) {
      const key: object = el ?? def;
      const stamp = stampOf(key, o.id, o.identity ?? "id");
      open.add(key);
      if (!shown) continue;
      out.push({
        def,
        el,
        key,
        over: safely(() => overOf(o, el), []),
        own: safely(() => connected(o.own?.(el ?? undefined)), []),
        regions: safely(() => connected(o.regions(el ?? undefined)), []),
        stamp,
      });
    }
  }
  for (const key of Array.from(stamps.keys())) {
    if (!open.has(key)) stamps.delete(key);
  }
  return out;
}

// overOf is the layer ids o names in over for el: a function is read for the
// element the layer's match selected, and anything in the answer that is not
// a string is dropped.
function overOf(o: LayerOptions, el: Element | null): string[] {
  const over = typeof o.over === "function" ? o.over(el ?? undefined) : o.over;
  return Array.from(over ?? []).filter((id): id is string => typeof id === "string");
}

function holds(outer: Instance, inner: Instance): boolean {
  return inner.regions.length > 0
    && inner.regions.every((region) => outer.regions.some((part) => part.contains(region)));
}

// above is whether layer goes above current: its regions lie inside
// current's, or it names current in over.
function above(layer: Instance, current: Instance): boolean {
  return (holds(current, layer) && !holds(layer, current)) || layer.over.includes(current.def.o.id);
}

// topLayer starts from the newest active layer and climbs to the newest layer
// that goes above it, and so on, visiting each layer once, so layers whose
// over names each other settle rather than take turns.
export function topLayer(all: Instance[]): Instance | null {
  let top: Instance | null = null;
  for (const layer of all) if (!top || layer.stamp > top.stamp) top = layer;
  const seen = new Set<Instance>();
  while (top) {
    seen.add(top);
    const current: Instance = top;
    let next: Instance | null = null;
    for (const layer of all) {
      if (!seen.has(layer) && above(layer, current) && (!next || layer.stamp > next.stamp)) next = layer;
    }
    if (!next) break;
    top = next;
  }
  return top;
}

// modalTop is the topmost of the active layers that ask to be modal, by the
// same stacking order topLayer reads among the rest, so a layer that does not
// ask, opened over a modal one, leaves the modal one deciding what is inert.
export function modalTop(all: Instance[]): Instance | null {
  return topLayer(all.filter((layer) => modal(layer)));
}

// aboveOf lists the active layers stacked over m, whose regions count as
// further regions of m for what is inert: every other layer m does not go
// above that is newer than m, or goes above it by its regions or its over,
// and top whenever top is not m, since top stands over every other layer.
export function aboveOf(all: Instance[], m: Instance, top: Instance | null): Instance[] {
  return all.filter((layer) =>
    layer !== m && !above(m, layer) && (layer === top || layer.stamp > m.stamp || above(layer, m))
  );
}

export function invokerOf(popover: Element): Element | null {
  const stamped = popovers.get(popover)?.invoker;
  if (stamped) return stamped;
  return popover.id ? document.querySelector('[popovertarget="' + CSS.escape(popover.id) + '"]') : null;
}

// inside is whether node belongs to the layer: in one of its regions, or in a
// popover opened from one, which shows in the top layer wherever it sits in
// the DOM.
export function inside(top: Instance, node: Node | null, depth = 0): boolean {
  if (!node) return false;
  if (top.regions.some((region) => region.contains(node))) return true;
  if (depth > 3 || !(node instanceof Element)) return false;
  const popover = node.closest(":popover-open");
  if (!popover) return false;
  const invoker = invokerOf(popover);
  return !!invoker && !popover.contains(invoker) && inside(top, invoker, depth + 1);
}

export function inOwn(top: Instance, node: Node): boolean {
  return top.own.some((part) => part.contains(node));
}

// walkStop is where a walk from target ends inside the top layer: the region
// holding it, or the popover it sits in.
export function walkStop(top: Instance, target: Element): Element {
  return top.regions.find((region) => region.contains(target)) ?? target.closest(":popover-open")
    ?? document.documentElement;
}

// addScope's handle also lets a script suspend and resume the entry without
// losing its stamp: a disable-Escape flag that toggles off again, as a
// popover's does, keeps the scope in its original place in the Escape order
// rather than jumping to the front on resume.
export function addScope(o: ScopeOptions): ScopeHandle {
  const entry: Dismissable = { close: o.close, element: o.element, invoker: null, stamp: ++clock };
  scopes.add(entry);
  return {
    close: () => {
      scopes.delete(entry);
    },
    resume: () => {
      entry.suspended = false;
    },
    suspend: () => {
      entry.suspended = true;
    },
  };
}

// newestDismissable is what the next Escape closes before the top layer's
// own Escape: the newest open popover or scope in the top layer or opened
// from it, or anywhere with no layer open. A layer's own element is left to
// the layer.
export function newestDismissable(top: Instance | null, all: Instance[]): Dismissable | null {
  sweep();
  // At a key no popover is part-way open, so one that is not open has closed
  // without a beforetoggle -- its popover attribute changed -- and goes.
  for (const el of Array.from(popovers.keys())) if (!el.matches(":popover-open")) popovers.delete(el);
  let best: Dismissable | null = null;
  for (const entry of [...popovers.values(), ...scopes]) {
    if (entry.suspended === true) continue;
    if (all.some((layer) => layer.el === entry.element)) continue;
    if (top && !inside(top, entry.element) && !(entry.invoker && inside(top, entry.invoker))) continue;
    if (!best || entry.stamp > best.stamp) best = entry;
  }
  return best;
}

// sweep drops the popovers and scopes whose element left the document, so the
// Escape stack never keeps removed nodes alive. A popover that closes leaves
// the stack by its beforetoggle; one still opening is kept, though it does
// not match :popover-open until its beforetoggle listeners have run, and a
// listener that changes the DOM settles the router before then.
export function sweep(): void {
  for (const el of Array.from(popovers.keys())) if (!el.isConnected) popovers.delete(el);
  for (const entry of Array.from(scopes)) if (!entry.element.isConnected) scopes.delete(entry);
}

export function dismiss(entry: Dismissable): void {
  popovers.delete(entry.element);
  scopes.delete(entry);
  safely(() => entry.close(), undefined);
}

// An auto popover joins the Escape stack as it opens, stamped with the
// element that opened it: the invoker the browser names, else the focused
// element. With neither -- a script opened it while focus was on the body --
// invokerOf finds it by popovertarget. The router closes it itself rather
// than leave it to the browser's light dismiss, which a synthetic Escape or a
// key in a frame never raises.
document.addEventListener(
  "beforetoggle",
  (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement) || target.popover !== "auto") return;
    const popover: HTMLElement = target;
    const toggle = event as ToggleEvent & { source?: Element | null };
    if (toggle.newState !== "open") {
      popovers.delete(popover);
      return;
    }
    const focused = document.activeElement;
    popovers.set(popover, {
      close: () => popover.hidePopover(),
      element: popover,
      invoker: toggle.source ?? (focused === document.body ? null : focused),
      stamp: ++clock,
    });
  },
  true,
);
