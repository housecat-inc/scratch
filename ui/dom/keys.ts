// The one key router. Every key handler registers here by selector, and a key
// walks from its target up through the target's ancestors, running the
// bindings it meets innermost first; the first handler that returns true
// claims the key, and the router prevents its default and stops it. A target
// that takes text resolves at <html>'s bubble phase, after the field's own
// listeners, so an editor's Command-B or a composer's Enter stays innermost;
// every other key resolves in the window's capture phase, ahead of any
// listener a page adds. That includes a key a script dispatches at the
// document while a field has focus: it is resolved for the field, but no
// field listener hears it and it never passes <html>.
//
// A registration an inline script makes is owned by that script: while a swap
// has the script out of the document the registration is skipped and then
// dropped, and when the script runs again it registers under the same id,
// replacing the old one. ui.js is itself a script while it runs, so it is set
// aside: a registration the bundle makes at import has no owner and stays for
// the page's life.

import { tabbable } from "tabbable";
import {
  altGraph,
  bareModifier,
  browserShortcut,
  type Chord,
  chordName,
  editingChord,
  matchChord,
  parseChord,
  reservedChord,
} from "./chord";
import { isElement } from "./dom";
import {
  droppedFrom,
  enterRing,
  noteAriaHidden,
  noteFocus,
  project,
  recover,
  restoreFocus,
  ringEdge,
} from "./keyfocus";
import { activates, composing, editable } from "./keyinfo";
import {
  aboveOf,
  active,
  addLayer,
  addScope,
  dismiss,
  inOwn,
  inside,
  type Instance,
  type LayerOptions,
  modalTop,
  newestDismissable,
  type ScopeHandle,
  type ScopeOptions,
  sweep,
  topLayer,
  walkStop,
  watchedAttributes,
} from "./keylayers";
import { misuse, safely } from "./keyreport";

export type Typing = "never" | "chords" | "always";

// A Handler gets the key, the element its binding's selector matched, and the
// element the key resolved to -- what has focus, for a key aimed at the
// document or the window -- and returns true to claim the key.
export type Handler = (event: KeyboardEvent, at: Element, target: Element) => boolean | void;

export type BindOptions = {
  controls?: boolean;
  frames?: boolean;
  id?: string;
  label?: string;
  owner?: Element;
  priority?: number;
  type?: "keydown" | "keyup";
  typing: Typing;
};

export type ObserveOptions = {
  id?: string;
  owner?: Element;
};

// Binding is a live registration as bindings() lists it, for the browser
// tests' manifest check.
export type Binding = {
  chords: string[] | "*";
  controls: boolean;
  frames: boolean;
  id: string;
  label: string;
  owned: boolean;
  priority: number;
  sel: string;
  type: "keydown" | "keyup";
  typing: Typing;
};

type Bound = {
  all: Handler | null;
  opts: BindOptions;
  owner: Element | null;
  pairs: { chord: Chord; handler: Handler; name: string }[];
  seq: number;
  sel: string;
};

type Watcher = {
  fn: (event: KeyboardEvent) => void;
  id: string | undefined;
  owner: Element | null;
};

const TYPING: readonly string[] = ["always", "chords", "never"];
const TYPES: readonly string[] = ["keydown", "keyup"];
// An attribute name holds no whitespace, quote, >, / or =; getAttribute takes
// any string, so a layer's identity that breaks this would never be found.
const ATTRIBUTE_NAME = /^[^\s"'>/=]+$/;
const bound: Bound[] = [];
const watchers: Watcher[] = [];
// Held is a key capture set aside for bubble, with the layers as capture saw
// them. bubble takes the frame as a parameter instead, since both of its call
// sites -- route() and the frame's own bubble listener -- know it statically.
type Held = {
  all: Instance[];
  target: Element;
  top: Instance | null;
};

const pending = new WeakMap<KeyboardEvent, Held>();
let seq = 0;

// ownerOf is the element a binding goes with: the one it names, or else the
// classic script running as it is made, an inline script a swap can take away.
// A module has no currentScript, so a binding a module makes on its own is
// the page's.
function ownerOf(given: Element | undefined): Element | null {
  return given ?? document.currentScript;
}

// selector is whether sel parses as a selector, so a typo is refused once,
// where it is registered, and not reported on every key.
function selector(sel: string): boolean {
  try {
    document.documentElement.matches(sel);
    return true;
  } catch {
    return false;
  }
}

function live(entry: { owner: Element | null }): boolean {
  return !entry.owner || entry.owner.isConnected;
}

// prune drops what a swap took out of the document, so nothing the router
// holds keeps removed nodes alive or calls into them.
function prune(): void {
  for (const list of [bound, watchers] as { owner: Element | null }[][]) {
    for (let i = list.length - 1; i >= 0; i--) if (!live(list[i])) list.splice(i, 1);
  }
}

export function on(sel: string, keys: Record<string, Handler> | Handler, o: BindOptions): () => void {
  // An inline script's call is untyped JS, which may leave the options out.
  const given = o as BindOptions | undefined;
  if (!given || !TYPING.includes(o.typing)) {
    throw new TypeError("HousecatKeys.on(" + JSON.stringify(sel) + "): typing must be never, chords or always");
  }
  if (o.type !== undefined && !TYPES.includes(o.type)) {
    throw new TypeError("HousecatKeys.on(" + JSON.stringify(sel) + "): type must be keydown or keyup");
  }
  if (!selector(sel)) throw new TypeError("HousecatKeys.on(" + JSON.stringify(sel) + "): not a selector");
  const owner = ownerOf(o.owner);
  if (owner && o.id === undefined) {
    misuse("an inline script's binding on " + sel + " has no id, so running the script again adds a second copy");
  }
  const entry: Bound = {
    all: typeof keys === "function" ? keys : null,
    opts: { ...o },
    owner,
    pairs: typeof keys === "function"
      ? []
      : Object.entries(keys).map(([text, handler]) => {
        const chord = parseChord(text);
        return { chord, handler, name: chordName(chord) };
      }),
    seq: ++seq,
    sel,
  };
  if (o.id !== undefined) {
    const at = bound.findIndex((b) => b.opts.id === o.id);
    if (at >= 0) bound.splice(at, 1);
  }
  bound.push(entry);
  return () => {
    const at = bound.indexOf(entry);
    if (at >= 0) bound.splice(at, 1);
  };
}

export function observe(fn: (event: KeyboardEvent) => void, o: ObserveOptions = {}): () => void {
  const owner = ownerOf(o.owner);
  if (owner && o.id === undefined) {
    misuse("an inline script's observer has no id, so running the script again adds a second copy");
  }
  const entry: Watcher = { fn, id: o.id, owner };
  if (o.id !== undefined) {
    const at = watchers.findIndex((w) => w.id === o.id);
    if (at >= 0) watchers.splice(at, 1);
  }
  watchers.push(entry);
  return () => {
    const at = watchers.indexOf(entry);
    if (at >= 0) watchers.splice(at, 1);
  };
}

export function bindings(): Binding[] {
  prune();
  return bound.map((b) => ({
    chords: b.all ? "*" : b.pairs.map((pair) => pair.name),
    controls: b.opts.controls === true,
    frames: b.opts.frames === true,
    id: b.opts.id ?? "",
    label: b.opts.label ?? "",
    owned: b.owner !== null,
    priority: b.opts.priority ?? 0,
    sel: b.sel,
    type: b.opts.type ?? "keydown",
    typing: b.opts.typing,
  }));
}

function normalize(event: KeyboardEvent, frame: HTMLIFrameElement | null): Element {
  if (frame) return frame;
  const target = event.target;
  if (target instanceof Element) return target;
  return document.activeElement ?? document.body;
}

// allows is whether a binding's typing policy lets it take event from a field:
// "chords" is a key with Control, Command or Alt, and Escape. AltGr text is
// typing, though Windows reports it with Control and Alt.
function allows(typing: Typing, event: KeyboardEvent): boolean {
  if (typing === "always") return true;
  if (typing === "never") return false;
  return ((event.ctrlKey || event.metaKey || event.altKey) && !altGraph(event)) || event.key === "Escape";
}

function handlerFor(b: Bound, event: KeyboardEvent): Handler | null {
  if (b.all) return b.all;
  return b.pairs.find((pair) => matchChord(pair.chord, event))?.handler ?? null;
}

function describe(b: Bound): string {
  return b.opts.id ?? b.sel;
}

// printable is a key that types a character: one with no modifier but Shift,
// or AltGr text.
function printable(event: KeyboardEvent): boolean {
  return event.key.length === 1 && (altGraph(event) || (!event.ctrlKey && !event.metaKey && !event.altKey));
}

// focusedIn is what has focus in a same-origin frame, which a key in the
// frame types into or presses though it walks from the <iframe>.
function focusedIn(frame: HTMLIFrameElement): Element | null {
  return safely(() => frame.contentDocument?.activeElement ?? null, null);
}

// walk runs the bindings on target and each ancestor up to stop, innermost
// first and, on one element, highest priority first. A binding matched at
// <body> or <html> never takes Enter or Space from a control that activates
// on them, unless it asks for controls, so a page's cursor never presses the
// row under it when a button has focus. It answers whether one claimed the
// key. A key in a frame answers the typing policy and that rule for what has
// focus in the frame, not for the <iframe>.
function walk(event: KeyboardEvent, target: Element, frame: HTMLIFrameElement | null, stop: Element): boolean {
  if (bareModifier(event) || editingChord(event) || reservedChord(event)) return false;
  const type = event.type === "keyup" ? "keyup" : "keydown";
  const from = frame ? focusedIn(frame) : target;
  const field = editable(from);
  for (let at: Element | null = target; at; at = at.parentElement) {
    const el: Element = at;
    // Bindings that share a selector, as a page's shortcuts do, match it once.
    const matched = new Map<string, boolean>();
    const matches = (sel: string): boolean => {
      const known = matched.get(sel);
      if (known !== undefined) return known;
      const found = safely(() => el.matches(sel), false);
      matched.set(sel, found);
      return found;
    };
    const here = bound
      .filter((b) => live(b) && (b.opts.type ?? "keydown") === type && matches(b.sel))
      .map((b) => ({ b, handler: handlerFor(b, event) }))
      .filter((h): h is { b: Bound; handler: Handler } => h.handler !== null)
      .sort((x, y) => (y.b.opts.priority ?? 0) - (x.b.opts.priority ?? 0) || x.b.seq - y.b.seq);
    for (let i = 1; i < here.length; i++) {
      if ((here[i].b.opts.priority ?? 0) === (here[i - 1].b.opts.priority ?? 0)) {
        misuse(
          describe(here[i - 1].b) + " and " + describe(here[i].b) + " both take " + JSON.stringify(event.key)
            + " on one element at one priority",
        );
      }
    }
    const page = el === document.body || el === document.documentElement;
    for (const { b, handler } of here) {
      if (field && !allows(b.opts.typing, event)) continue;
      if (page && b.opts.controls !== true && (event.key === "Enter" || event.key === " ") && activates(from)) continue;
      if (frame && b.opts.frames !== true) continue;
      if (!safely(() => handler(event, el, target) === true, false)) continue;
      if (field && b.opts.typing === "always" && el !== target && printable(event)) {
        misuse(describe(b) + " took the typed key " + JSON.stringify(event.key) + " from a field");
      }
      return true;
    }
    if (el === stop) break;
  }
  return false;
}

// claimed holds the code of each keydown the router claimed, until its keyup,
// which is prevented too: a claimed Space never goes on to press the button
// focus has moved to by the time it comes back up.
const claimed = new Set<string>();

function claim(event: KeyboardEvent): void {
  event.preventDefault();
  event.stopPropagation();
  if (event.type === "keydown") claimed.add(event.code);
}

// dropped holds the code of each keydown kept from the page because it was
// aimed outside the top layer. Focus has gone back into the layer by the time
// its keyup comes, so the keyup is answered by its keydown, not its target.
const dropped = new Set<string>();

// keyupOf answers a keyup the way its keydown was answered: a claimed key's
// keyup is prevented and still propagates, a dropped key's is kept from the
// page too. It answers whether the keyup needs nothing more.
function keyupOf(event: KeyboardEvent): boolean {
  if (claimed.delete(event.code)) event.preventDefault();
  if (!dropped.delete(event.code)) return false;
  event.stopPropagation();
  event.preventDefault();
  return true;
}

function notify(event: KeyboardEvent): void {
  for (const w of watchers.filter(live)) {
    const before = event.defaultPrevented;
    safely(() => w.fn(event), undefined);
    if (event.defaultPrevented && !before) {
      misuse("an observer cancelled " + JSON.stringify(event.key) + "; an observer watches keys and never claims one");
    }
  }
}

// towardField is whether a key is on its way to a field, whose own listeners
// hear it before <html> does: the document's own target in this document, or
// -- mirroring that split -- the frame's focused element for a frame key. A
// key aimed at the document or the window, as a script may dispatch one,
// passes neither a field nor <html>, so it is not: it resolves at once, for
// whatever has focus. This holds inside a frame too: a key a script dispatches
// straight at the frame's document never reaches that document's <html>, so
// deferring it there would drop it for good.
function towardField(event: KeyboardEvent, target: Element, frame: HTMLIFrameElement | null): boolean {
  // A frame event's own target is that frame's Element, not this window's,
  // so instanceof would always be false; nodeType survives the realm.
  const aimed = frame ? isElement(event.target) : event.target instanceof Element;
  return aimed && editable(frame ? focusedIn(frame) : target);
}

// capture runs in the window's capture phase, before any listener a page
// adds, and in the capture phase of an adopted frame's document. A keyup is
// first answered the way its keydown was. It finds the top layer, keeps a key
// aimed outside it from the page, and resolves the key now unless it is on
// its way to a field.
function capture(event: KeyboardEvent, frame: HTMLIFrameElement | null): void {
  if (event.type === "keydown") notify(event);
  else if (keyupOf(event)) return;
  if (composing(event)) return;
  prune();
  const all = active();
  const top = topLayer(all);
  const target = normalize(event, frame);
  if (top && !inside(top, target)) return outside(event, top, all);
  const own = top !== null && inOwn(top, target);
  if (!own && towardField(event, target, frame)) {
    pending.set(event, { all, target, top });
    return;
  }
  resolve(event, target, top, all, frame);
}

// bubble runs at <html>'s bubble phase -- this document's, or an adopted
// frame's -- for a key whose target takes text, once the field's own
// listeners have run. One that prevented the key's default has claimed it,
// and inside a layer the key goes no further. frame is the one bubble's own
// listener is bound in, null at the top level, same as capture's.
function bubble(event: KeyboardEvent, frame: HTMLIFrameElement | null): void {
  const held = pending.get(event);
  if (!held) return;
  pending.delete(event);
  if (!event.defaultPrevented) return resolve(event, held.target, held.top, held.all, frame);
  if (held.top) event.stopPropagation();
}

// resolve walks the key from its target, runs the Escape order, moves Tab
// across the top layer's parts at a part's edge, and seals what is left
// inside the top layer against the page. A sealed key is stopped, not
// cancelled, so a focused button still presses and a field still types.
function resolve(
  event: KeyboardEvent,
  target: Element,
  top: Instance | null,
  all: Instance[],
  frame: HTMLIFrameElement | null,
): void {
  if (walk(event, target, frame, top ? walkStop(top, target) : document.documentElement)) return claim(event);
  if (event.type === "keydown" && event.key === "Escape" && escapeOrder(event, top, all)) return claim(event);
  if (top && event.type === "keydown" && tab(event) && ringEdge(top, tabFrom(target, frame), event.shiftKey, frame)) {
    return claim(event);
  }
  if (top) event.stopPropagation();
}

// tabFrom is where a Tab the walk left unclaimed moves on from: where focus
// is now, which a binding may have moved without claiming the key -- a menu
// hands focus back to its trigger on Tab so the browser moves on from there
// -- else the key's target.
function tabFrom(target: Element, frame: HTMLIFrameElement | null): Element {
  const focused = document.activeElement;
  return !frame && focused && focused !== document.body ? focused : target;
}

// outside keeps a key aimed outside the top layer from the page. Tab enters
// the layer and Escape runs its Escape order; anything else is stopped,
// cancelled unless the browser answers it -- find, zoom, reload -- and focus
// goes back into the layer. A modifier pressed on its own is only stopped, so
// holding Shift for Shift+Tab does not move focus first.
function outside(event: KeyboardEvent, top: Instance, all: Instance[]): void {
  const down = event.type === "keydown";
  event.stopPropagation();
  if (bareModifier(event)) return;
  if (down && tab(event)) {
    // Focus that fell to the body goes back to where it was first, and Tab
    // moves on from there: by the browser's own order inside a part, across
    // the ring at a part's edge. From anywhere else, or when that element no
    // longer takes focus, Tab enters the layer at its first stop, or with
    // Shift its last.
    const from = droppedFrom(top);
    from?.focus({ preventScroll: true });
    if (!from || document.activeElement !== from) {
      claim(event);
      enterRing(top, event.shiftKey);
      return;
    }
    if (ringEdge(top, from, event.shiftKey, null)) claim(event);
    return;
  }
  if (down && event.key === "Escape" && escapeOrder(event, top, all)) return claim(event);
  if (!browserShortcut(event)) event.preventDefault();
  if (!down) return;
  dropped.add(event.code);
  restoreFocus(top);
}

function tab(event: KeyboardEvent): boolean {
  return event.key === "Tab" && !event.ctrlKey && !event.metaKey && !event.altKey;
}

// escapeOrder runs what an Escape the walk left unclaimed does: close the
// newest dismissable in the top layer, else run the top layer's Escape. It
// answers whether either took the key.
function escapeOrder(event: KeyboardEvent, top: Instance | null, all: Instance[]): boolean {
  const next = newestDismissable(top, all);
  if (next) {
    dismiss(next);
    return true;
  }
  if (!top) return false;
  const { def, el } = top;
  return safely(() => def.o.escape(event, el ?? undefined) === true, false);
}

const medias = new Set<string>();
// The observer settles the router after every mutation batch; first it reads
// the batch's aria-hidden changes against the elements it has hidden, since
// settle's own recompute writes aria-hidden too, and a write it makes before
// the batch is read would be taken for one the batch holds.
const observer = new MutationObserver((records) => {
  noteAriaHidden(records);
  settle();
});

type Closer = { destroy: () => void; onclose: (() => void) | null };

// watching is the one CloseWatcher the router holds and the key of the layer
// it holds it for.
let watching: { closer: Closer; key: object } | null = null;

// backLayer is the topmost of the active layers that ask for back, by the
// same stacking order topLayer reads among the rest, which gives the
// watcher its armed identity: a layer stacked above it that does not ask for
// back, such as a dialog or the screenshot inspector, never tears it down.
function backLayer(all: Instance[]): Instance | null {
  return topLayer(all.filter((layer) => layer.def.o.back === true));
}

// watch gives one CloseWatcher to the topmost active layer that asks for
// back, when the pointer is coarse, so Android's back gesture reaches a
// drawer even under a layer that does not ask for it, such as a dialog or
// the screenshot inspector. Its onclose runs one step of the Escape order at
// the actual top layer, whichever that is -- there is no key: the newest
// dismissable there, else that layer's own Escape, the same step one Escape
// key press would take -- and re-arms for whatever back:true layer remains
// open, reading the DOM fresh since that step may have closed the layer
// above the drawer, the drawer itself, or neither. Escape itself is resolved
// and cancelled in keydown, so the watcher only ever sees back. Where
// CloseWatcher is missing, back does what the browser does.
function watch(all: Instance[]): void {
  const Watcher = window.CloseWatcher;
  const want = Watcher && matchMedia("(pointer: coarse)").matches ? backLayer(all) : null;
  if ((want?.key ?? null) === (watching?.key ?? null)) return;
  watching?.closer.destroy();
  watching = null;
  if (!want || !Watcher) return;
  // The constructor can throw -- per spec, when the document is not fully
  // active -- and a settle that keeps running matters more than one watcher.
  const closer = safely(() => new Watcher(), null);
  if (!closer) return;
  const held = { closer, key: want.key };
  held.closer.onclose = () => {
    if (watching === held) watching = null;
    const current = active();
    escapeOrder(new KeyboardEvent("keydown", { key: "Escape" }), topLayer(current), current);
    watch(active());
  };
  watching = held;
}

// projectFrom projects the stack: aria-hidden from top, and inert from the
// topmost modal layer with every layer stacked over it.
function projectFrom(all: Instance[], top: Instance | null): void {
  const held = modalTop(all);
  project(top, held, held ? aboveOf(all, held, top) : []);
}

// settle brings the router up to date with the DOM after a mutation batch, a
// toggle or a media change: it drops what a swap removed, from bindings to
// the Escape stack, and stamps the layers that just became active, so their
// order never comes from key history; it hides what lies outside the top
// layer from assistive technology and holds inert what lies outside the
// topmost layer that asks and the layers over it; and it brings focus back
// into the top layer when the element focused there was removed or the
// projection took it, or, with no layer open, to the nearest control when a
// swap took the focused one. The projection runs ahead of the focus step so
// that step sees it. Exported for a script that writes a layer's record and
// then focuses or reads the page in the same task, as a drawer's open() does:
// the observer's own settle comes on the batch's microtask, after that task.
export function settle(): void {
  prune();
  sweep();
  const all = active();
  const top = topLayer(all);
  projectFrom(all, top);
  recover(top);
  watch(all);
}

function watchLayers(): void {
  observer.observe(document.documentElement, {
    attributeFilter: watchedAttributes(),
    attributeOldValue: true,
    attributes: true,
    childList: true,
    subtree: true,
  });
}

export function layer(o: LayerOptions): void {
  // An inline script's call is untyped JS, which may leave the options out.
  const given = o as LayerOptions | undefined;
  if (!given || typeof o.id !== "string" || typeof o.regions !== "function" || typeof o.escape !== "function") {
    throw new TypeError("HousecatKeys.layer: id, regions and escape are required");
  }
  if (o.match !== undefined && !selector(o.match)) {
    throw new TypeError("HousecatKeys.layer(" + JSON.stringify(o.id) + "): match is not a selector");
  }
  if (o.identity !== undefined && (typeof o.identity !== "string" || !ATTRIBUTE_NAME.test(o.identity))) {
    throw new TypeError("HousecatKeys.layer(" + JSON.stringify(o.id) + "): identity must name an attribute");
  }
  if (o.modal !== undefined && typeof o.modal !== "boolean" && typeof o.modal !== "function") {
    throw new TypeError("HousecatKeys.layer(" + JSON.stringify(o.id) + "): modal must be a boolean or a function");
  }
  if (o.over !== undefined && !Array.isArray(o.over) && typeof o.over !== "function") {
    throw new TypeError("HousecatKeys.layer(" + JSON.stringify(o.id) + "): over must be a list or a function");
  }
  addLayer({ ...o }, ownerOf(o.owner));
  const media = o.media;
  if (media && !medias.has(media)) {
    medias.add(media);
    matchMedia(media).addEventListener("change", settle);
  }
  watchLayers();
  settle();
}

// scope refuses an element that is not one, which the Escape stack would
// otherwise read on every Escape and every mutation batch, and throw there.
// One in a same-origin frame is that frame's Element, so it is known by its
// nodeType.
export function scope(o: ScopeOptions): ScopeHandle {
  // An inline script's call is untyped JS, which may leave the options out.
  const given = o as ScopeOptions | undefined;
  if (!given || (o.element as Node | undefined)?.nodeType !== 1 || typeof o.close !== "function") {
    throw new TypeError("HousecatKeys.scope: element and close are required");
  }
  return addScope({ ...o });
}

const framed = new WeakSet<Document>();
const armed = new WeakSet<HTMLIFrameElement>();

// A key in a same-origin frame, such as a message body, never reaches this
// document, so the router listens on the frame's own document once focus goes
// into it -- the window blurs -- and again each time it loads. Its keys run
// with the <iframe> as their target and reach only bindings that ask for
// frames, each under its typing policy for what has focus in the frame;
// Escape still runs the Escape order.
function adopt(frame: HTMLIFrameElement): void {
  if (!armed.has(frame)) {
    armed.add(frame);
    frame.addEventListener("load", () => bindFrame(frame));
  }
  bindFrame(frame);
}

function bindFrame(frame: HTMLIFrameElement): void {
  const doc = safely(() => frame.contentDocument, null);
  // A document may have no root element, which TypeScript's DOM types leave out.
  const root = doc?.documentElement as HTMLElement | null | undefined;
  if (!doc || !root || framed.has(doc)) return;
  framed.add(doc);
  const captured = tag((event: KeyboardEvent) => capture(event, frame));
  doc.addEventListener("keydown", captured, true);
  doc.addEventListener("keyup", captured, true);
  // Mirrors the top-level split: a key capture sets aside for a field inside
  // the frame resolves here, at the frame's own <html> bubble phase, once the
  // field's own listeners -- bound inside the frame, so only this phase runs
  // after them -- have had the key.
  const bubbled = tag((event: KeyboardEvent) => bubble(event, frame));
  root.addEventListener("keydown", bubbled);
  root.addEventListener("keyup", bubbled);
}

function route(event: Event): void {
  if (!(event instanceof KeyboardEvent)) return;
  if (event.currentTarget === window) capture(event, null);
  else bubble(event, null);
}

// tag marks one of the router's own listeners, which the browser tests'
// listener census leaves out.
function tag<T extends object>(fn: T): T {
  Object.defineProperty(fn, "housecatKeys", { value: true });
  return fn;
}

// Queued is a call KeysStub took before this module ran: its kind and
// arguments, the script that made it, whether that script has since taken it
// back, what the real call returned, which the stub's handle forwards to, and
// -- for a scope -- whether the stub's handle was told to suspend before the
// real one existed to tell.
// Its arguments are the ones the stub's function of that kind was given,
// which are the real function's.
type Queued =
  & {
    done: unknown;
    dropped: boolean;
    owner: Element | null;
    suspended?: boolean;
  }
  & (
    | { args: Parameters<typeof layer>; kind: "layer" }
    | { args: Parameters<typeof observe>; kind: "observe" }
    | { args: Parameters<typeof on>; kind: "on" }
    | { args: Parameters<typeof scope>; kind: "scope" }
  );

// withOwner gives a queued call's options the script that queued it as owner,
// unless the call named its own.
function withOwner<T extends { owner?: Element }>(o: T, owner: Element | null): T {
  return o.owner || !owner ? o : { ...o, owner };
}

// drain makes the calls KeysStub queued, each owned by the script that made
// it unless the call named its own owner.
function drain(queue: Queued[]): void {
  for (const call of queue.splice(0)) {
    if (call.dropped) continue;
    safely(() => {
      if (call.kind === "on") {
        const [selector, handlers, o] = call.args;
        call.done = on(selector, handlers, withOwner(o, call.owner));
      } else if (call.kind === "observe") {
        const [fn, o] = call.args;
        call.done = observe(fn, withOwner(o ?? {}, call.owner));
      } else if (call.kind === "layer") {
        layer(withOwner(call.args[0], call.owner));
      } else {
        const handle = scope(call.args[0]);
        if (call.suspended === true) handle.suspend();
        call.done = handle;
      }
    }, undefined);
  }
}

declare global {
  interface HousecatKeysGlobal {
    bindings: typeof bindings;
    browserShortcut: typeof browserShortcut;
    layer: typeof layer;
    observe: typeof observe;
    on: typeof on;
    queue?: Queued[];
    route?: typeof route;
    scope: typeof scope;
    settle: typeof settle;
    stubbed?: boolean;
    tabbable: typeof tabbable;
  }
  interface Window {
    // Missing where the browser has no CloseWatcher, and from TypeScript's DOM types.
    CloseWatcher?: new() => Closer;
  }
}

export const keys = Object.assign(window.HousecatKeys, {
  bindings,
  browserShortcut,
  layer,
  observe,
  on,
  route,
  scope,
  settle,
  tabbable,
});
if (keys.stubbed !== true) {
  const listener = tag(route);
  window.addEventListener("keydown", listener, true);
  window.addEventListener("keyup", listener, true);
  document.documentElement.addEventListener("keydown", listener);
  document.documentElement.addEventListener("keyup", listener);
}
watchLayers();
document.addEventListener("toggle", settle, true);
matchMedia("(pointer: coarse)").addEventListener("change", settle);
// The layer remembers where focus last was in it, which is where a stray key
// sends focus back. A press on text that takes no focus, or a script's
// blur(), drops focus to the body; it stays there until the next key, which
// outside() answers, so nothing listens for focusout. The marks that hide
// what lies outside the layer are recomputed at once, so the element taking
// focus is never under one.
document.addEventListener("focusin", (event) => {
  const all = active();
  const top = topLayer(all);
  noteFocus(top, event.target);
  projectFrom(all, top);
});
// Focus going into a frame blurs this window and focuses the <iframe> here;
// either one adopts the frame.
window.addEventListener("blur", () => {
  claimed.clear();
  dropped.clear();
  setTimeout(() => {
    const focused = document.activeElement;
    if (focused instanceof HTMLIFrameElement) adopt(focused);
  });
});
document.addEventListener("focusin", (event) => {
  if (event.target instanceof HTMLIFrameElement) adopt(event.target);
});
drain(keys.queue ?? []);
settle();
