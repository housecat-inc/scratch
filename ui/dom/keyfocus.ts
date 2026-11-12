// Focus inside the top modal layer, for the router in keys.ts.
//
// Tab moves between the layer's parts only at their edges: inside a part, and
// in a popover opened from one, the browser's own order runs, so an editor's
// own Tab and a scroller the keyboard can reach keep working. Focus that
// strays outside the layer -- a script's deliberate focus() elsewhere, or a
// press on text that drops it to the body -- stands until the next key, which
// is kept from the page and brings focus back to where it last was in the
// layer; a Tab moves on from there. Focus comes back at once only when the
// focused element is removed.
// Everything outside the top layer but the element with focus is hidden
// from assistive technology with aria-hidden. Inert follows the topmost
// layer that asks for it (modal), which need not be the top layer: the
// layers stacked over it count as its regions, so a dialog or a popover
// opened over the sheet leaves the page under the sheet inert and itself
// live. With no modal layer nothing is inert: inert would stop a backdrop
// outside a layer taking a tap, so a layer that asks lists its backdrop
// among its regions. The inert walk covers the element with focus too --
// Chrome refuses aria-hidden above a focused element, while inert on its
// ancestor blurs it, and recover() then brings focus into the top layer --
// and never a live region or a popover opened from inside the modal layer
// or one over it. Each is marked (data-keys-hidden, data-keys-inert) as the
// router's own, and the router only ever removes its own: an element the
// author hid keeps its aria-hidden, put back when it is shown again, and an
// element the author made inert is never touched.
//
// Tab stops are tabbable's, plus any focusable same-origin iframe, which
// tabbable leaves out entirely, so a part that ends in one extends into it.
// The ring reaching a frame lands inside it, and a Tab from inside one moves
// the ring only off the frame's own edge; short of that, it is left to the
// browser, which moves focus between the frame's own stops. A cross-origin
// frame is no stop: the router hears no key inside one, so Tab out of it
// would follow the page's order, not the ring. A focusable scroller is
// tabbable's blind spot, so a part whose last stop is one of those ends
// before it.

import { getTabIndex, isFocusable, tabbable } from "tabbable";
import { covered, type Marks, nearest, type Place, place, reconcile, type Tree } from "./keycover";
import { editable } from "./keyinfo";
import { inside, type Instance, invokerOf } from "./keylayers";
import { safely } from "./keyreport";

type Stop = HTMLElement | SVGElement;

const STOPS = { displayCheck: "full", includeContainer: true } as const;
const HIDDEN = "data-keys-hidden";
const LIVE = '[aria-live]:not([aria-live="off"]), [role=alert], [role=log], [role=status]';
const QUIET = new Set(["LINK", "SCRIPT", "STYLE", "TEMPLATE"]);
const INERT = "data-keys-inert";
// LANDING marks where focus lands when a swap takes the focused element out
// of a region: the marked element itself, or, on a container such as a
// navigable list, its first Tab stop -- the list's current row. The mark,
// data-focus="landing", sits only on the elements it marks.
const LANDING = '[data-focus="landing"]';

// DOM reads the document as keycover's Tree; quiet is what the walks never
// list or enter; attributes writes the bare marks and inert.
const DOM: Tree<Element> = {
  children: (el) => Array.from(el.children),
  parent: (el) => el.parentElement,
};
const quiet = (el: Element): boolean => QUIET.has(el.tagName);
const attributes: Marks<Element> = {
  has: (el, name) => el.hasAttribute(name),
  remove: (el, name) => el.removeAttribute(name),
  set: (el, name) => el.setAttribute(name, ""),
};

// last is where focus last was in each layer, for the layer's stamp then: a
// layer that closes and opens again is stamped anew and starts afresh.
const last = new WeakMap<object, { el: Stop; stamp: number }>();
// noted is the element focus last went to while it was in the top layer, and
// that layer's stamp then, until focus goes somewhere outside the layer. Its
// removal brings focus back into that layer even when a swap replaced the
// layer's own element and took it along, and never into a layer that opened
// since.
let noted: { el: Stop; stamp: number } | null = null;
// based is the element focus last went to, wherever that was, and where it
// sat then, for recovery with no layer open and for a modal layer opening
// over it.
let based: { el: Stop; place: Place<Element> } | null = null;

// frameIsStop is whether frame is a same-origin stop: in the tab order and
// focusable by tabbable's own measure, which its focusable candidates, unlike
// its tabbable ones, extend to iframes.
function frameIsStop(frame: HTMLIFrameElement): boolean {
  return getTabIndex(frame) >= 0 && isFocusable(frame, STOPS) && safely(() => frame.contentDocument, null) !== null;
}

// stops are a part's Tab stops: tabbable's, plus any iframe among them it
// leaves out, each frame placed where tabbable's own order puts an element
// with its tabindex. A key from inside an adopted frame resolves with the
// <iframe> as its target (keys.ts's tabFrom), so the ring needs it in the list
// to find the part holding it and to tell whether it is that part's edge --
// ringEdge decides that itself, from the frame's own focus, before this list
// is ever asked about the frame.
function stops(part: Element): Stop[] {
  const list: Stop[] = tabbable(part, STOPS);
  for (const frame of Array.from(part.querySelectorAll("iframe")).filter(frameIsStop)) {
    const at = list.findIndex((stop) => tabsBefore(frame, stop));
    list.splice(at < 0 ? list.length : at, 0, frame);
  }
  return list;
}

// tabsBefore is whether a comes ahead of b in tabbable's order: a positive
// tabindex first, lowest first, then the rest, each in document order.
function tabsBefore(a: Stop, b: Stop): boolean {
  const rank = (el: Stop): number => (getTabIndex(el) > 0 ? getTabIndex(el) : Infinity);
  return rank(a) < rank(b) || (rank(a) === rank(b) && follows(a, b));
}

// follows is whether b comes after a in the document.
function follows(a: Node, b: Node): boolean {
  return (a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING) !== 0;
}

// land focuses a stop the ring reaches. A frame takes focus at its first stop
// inside, or with Shift its last: focus on the <iframe> itself leaves the
// frame's body focused, and the next Tab, which resolves at the <iframe>,
// would read as the part's edge and cross the ring past everything in it.
function land(to: Stop, back: boolean): void {
  const doc = to instanceof HTMLIFrameElement ? safely(() => to.contentDocument, null) : null;
  const inner = doc?.body ? tabbable(doc.body, STOPS) : [];
  const first = back ? inner.at(-1) : inner.at(0);
  (first ?? to).focus();
}

// frameAtEdge is whether a Tab resolved from inside frame is at frame's own
// edge: its document's last tabbable for Tab, or with Shift its first. A key
// from inside an adopted frame always resolves with the <iframe> as its
// target (keys.ts's tabFrom), since the router cannot see past it without
// asking; off that edge the key is left false here, for the browser's own
// Tab to move between the frame's stops, same as it would with no router in
// the page at all. A frame with no tabbable stop of its own -- empty, or not
// yet loaded -- has no inside to stay within, so it is always its own edge.
function frameAtEdge(frame: HTMLIFrameElement, back: boolean): boolean {
  const doc = safely(() => frame.contentDocument, null);
  const inner = doc?.body ? tabbable(doc.body, STOPS) : [];
  const edge = back ? inner.at(0) : inner.at(-1);
  return !edge || doc?.activeElement === edge;
}

// ringEdge moves focus on when Tab leaves target's part: to the next part's
// first stop, or with Shift the previous part's last, wrapping, and scrolls
// it into view. It answers false while a stop remains in the part that way,
// leaving the key to the browser, and for a target in a popover opened from a
// part, wherever that popover sits, whose own order the browser keeps. A key
// from inside frame moves the ring only from frame's own edge; off it, the
// key is left false too, for the frame's own Tab to run.
export function ringEdge(top: Instance, target: Element, back: boolean, frame: HTMLIFrameElement | null): boolean {
  if (frame && !frameAtEdge(frame, back)) return false;
  const parts = top.regions;
  const at = parts.findIndex((part) => part.contains(target));
  if (at < 0) return inside(top, target) ? false : enterRing(top, back);
  const here = stops(parts[at]);
  const onward = here.some((stop) => stop !== target && (back ? follows(stop, target) : follows(target, stop)));
  if (onward) return false;
  for (let step = 1; step <= parts.length; step++) {
    const list = step === parts.length ? here : stops(parts[(at + (back ? parts.length - step : step)) % parts.length]);
    const to = back ? list.at(-1) : list.at(0);
    if (to) {
      land(to, back);
      return true;
    }
  }
  return true;
}

// enterRing sends focus from outside the layer to its first stop, or with
// Shift its last, and scrolls it into view.
export function enterRing(top: Instance, back: boolean): boolean {
  const all = top.regions.flatMap(stops);
  const to = back ? all.at(-1) : all.at(0);
  if (to) land(to, back);
  return true;
}

export function noteFocus(top: Instance | null, target: EventTarget | null): void {
  noted = null;
  if (!(target instanceof HTMLElement || target instanceof SVGElement)) return;
  based = { el: target, place: place(target, DOM) };
  if (top && inside(top, target)) {
    last.set(top.key, { el: target, stamp: top.stamp });
    noted = { el: target, stamp: top.stamp };
  }
}

// lastIn is where focus last was in top while it has been open.
function lastIn(top: Instance): Stop | null {
  const entry = last.get(top.key);
  return entry && entry.stamp === top.stamp ? entry.el : null;
}

// restoreFocus puts focus back in the layer: on the element last focused in
// it, else on the layer's initial(), else on its first stop. Each is looked
// for only when the one before it fails. With typing false the first two
// are passed over when they take typing, and the first stop that takes none
// comes before the first stop: a projection that takes focus asks for no
// typing, because a text field focused by no gesture raises a phone's
// keyboard over the view the user just opened. The stops are listed once,
// since each listing is a tabbable pass over every region.
export function restoreFocus(top: Instance, typing = true): void {
  let listed: Stop[] | null = null;
  const all = (): Stop[] => (listed ??= top.regions.flatMap(stops));
  const typed = (el: Stop | null | undefined): Stop | null => (el && (typing || !editable(el)) ? el : null);
  const candidates: (() => Stop | null | undefined)[] = [
    () => typed(lastIn(top)),
    () => typed(safely(() => top.def.o.initial?.(top.el ?? undefined) ?? null, null)),
    () => (typing ? null : all().find((stop) => !editable(stop))),
    () => all()[0],
  ];
  for (const candidate of candidates) {
    const el = candidate();
    if (!el || !el.isConnected || !inside(top, el)) continue;
    el.focus({ preventScroll: true });
    if (document.activeElement === el) return;
  }
}

// droppedFrom is where focus was in the layer when it has since fallen to the
// body -- a press on text that takes no focus, or a script's blur() -- and
// that element is still there to go back to; else null.
export function droppedFrom(top: Instance): Stop | null {
  const current = document.activeElement;
  if (current && current !== document.body) return null;
  const was = lastIn(top);
  return was && was.isConnected && inside(top, was) ? was : null;
}

// recover answers a mutation batch or a projection. In a layer: when the
// batch took the element last focused in the layer out of the document, or
// a swap replaced the layer's own element and took it along, focus has
// dropped to the body without a focusout in every browser, and goes back
// into the layer; and when the projection has made the focused element's
// ancestor inert, which the browser blurs on its own schedule, focus goes
// into the layer at once, so the element still focused and the body it has
// fallen from read as one case. With no layer: when the element focus last
// went to is disconnected, hidden or inert and focus has fallen to the body,
// it moves to the nearest control in the same batch. Only focus on the body
// is ever moved, so a script's own hand-off, made before the batch settles,
// wins.
export function recover(top: Instance | null): void {
  if (!top) return recoverBase();
  const current = document.activeElement;
  const onBody = !current || current === document.body;
  const held = onBody ? based?.el ?? null : current;
  if (held?.closest("[" + INERT + "]")) {
    restoreFocus(top, false);
    return;
  }
  if (!onBody) return;
  const was = lastIn(top);
  const lost = noted !== null && noted.stamp === top.stamp && !noted.el.isConnected;
  if ((!was || was.isConnected) && !lost) return;
  noted = null;
  restoreFocus(top);
}

// recoverBase moves focus that fell to the body to the landing the swapped
// content declares, else to the control nearest where it was (keycover's
// nearest), among the page's tabbable stops, which leave out what is hidden
// or inert.
function recoverBase(): void {
  const current = document.activeElement;
  if (!based || (current && current !== document.body)) return;
  const { el, place: at } = based;
  if (el.isConnected && isFocusable(el, STOPS)) return;
  based = null;
  const to = landing(at) ?? nearest(at, tabbable(document.body, STOPS), DOM);
  if (to instanceof HTMLElement || to instanceof SVGElement) to.focus({ preventScroll: true });
}

// landing is the stop the content swapped in around where lost sat declares
// for focus: the deepest of lost's ancestors still in the document is the
// region the swap replaced, and the first landing in it, in document order,
// gives the stop -- itself when it is one, else its first Tab stop. A region
// that declares none leaves the choice to nearest.
function landing(lost: Place<Element>): Element | null {
  const region = lost.chain.filter((el) => el.isConnected).at(-1);
  if (!region) return null;
  const marked = region.matches(LANDING) ? [region] : Array.from(region.querySelectorAll(LANDING));
  for (const el of marked) {
    const stop = tabbable(el, STOPS).at(0);
    if (stop) return stop;
  }
  return null;
}

// strayFocus is the element with focus when it lies outside the layer and
// holds none of its regions, which project leaves audible; one that holds a
// region is never marked, and what it holds outside the layer can be.
function strayFocus(top: Instance): Element[] {
  const focused = document.activeElement;
  if (!focused || focused === document.body || inside(top, focused)) return [];
  return top.regions.some((region) => focused.contains(region)) ? [] : [focused];
}

// ariaWritten is every element whose aria-hidden the router has set to "true"
// since the last mutation batch was read.
const ariaWritten = new WeakSet<Element>();

// noteAriaHidden reads an aria-hidden the author set to "true" on an element
// the router has hidden, which becomes the value project() puts back when the
// element is shown again. Any other value the author sets there, project()
// keeps itself, synchronously, before it hides the element again; "true" is
// the one value it cannot tell from its own. An element's last record in the
// batch decides: one that turned some other value to "true" while the router
// had written is the router's own write, since project() only writes over a
// value that is not "true". A record batch can hold the author's write and
// the router's after it, when a script's focus() runs project() before the
// batch is delivered.
export function noteAriaHidden(records: MutationRecord[]): void {
  const lastOf = new Map<Element, MutationRecord>();
  for (const record of records) {
    if (record.attributeName === "aria-hidden" && record.target instanceof Element) lastOf.set(record.target, record);
  }
  for (const [el, record] of lastOf) {
    const written = ariaWritten.delete(el);
    const value = el.getAttribute("aria-hidden");
    if (!el.hasAttribute(HIDDEN) || (written && value === "true" && record.oldValue !== "true")) continue;
    el.setAttribute(HIDDEN, value ?? "");
  }
}

// liveRegions are the page's live regions, which every walk leaves alone.
function liveRegions(): Element[] {
  return Array.from(document.querySelectorAll(LIVE));
}

// openedFrom lists the open popovers whose invoker sits inside one of layers.
function openedFrom(layers: readonly Instance[]): Element[] {
  return Array.from(document.querySelectorAll(":popover-open")).filter((popover) => {
    const invoker = invokerOf(popover);
    return !!invoker && layers.some((layer) => inside(layer, invoker));
  });
}

// project hides from assistive technology everything outside the top
// layer's regions: each sibling of a region or of one of its ancestors
// (keycover's covered) gets aria-hidden and data-keys-hidden, the mark that
// says the router set it and holds the value to put back -- the author's,
// from before the element was hidden or set while it was, or "" for none. A
// live region stays audible, as does a popover opened from inside the
// layer, and an element the author hid keeps its own aria-hidden. The
// element with focus stays audible wherever it is, since Chrome refuses
// aria-hidden above a focused element and exposes that subtree for good.
// held is the topmost layer that asks to be modal and over the layers
// stacked above it: the same walk over held's regions and theirs, with live
// regions and the popovers opened from inside any of them kept and the
// focused element covered, decides what is inert, marked data-keys-inert;
// an element inert without the mark is the author's. The marks are
// recomputed on every focusin, ahead of that check, and on every mutation
// batch, so a swap or a morph that strips them gets them back. The router
// only ever removes its own.
//
// A marked element whose aria-hidden is not "true" has had it set by someone
// else since the router wrote it, so that value is the one to keep: stored in
// the mark when the element stays hidden, left in place when it is shown.
// This holds whether or not the mutation batch has been read yet, since a
// focusin runs project() ahead of the batch.
export function project(top: Instance | null, held: Instance | null, over: readonly Instance[]): void {
  const live = liveRegions();
  let inert: Element[] = [];
  if (held && held.regions.length > 0) {
    const layers = [held, ...over];
    const kept = [...layers.flatMap((layer) => layer.regions), ...live, ...openedFrom(layers)];
    inert = covered(document.body, kept, DOM, quiet);
  }
  const hidden = new Set<Element>();
  if (top && top.regions.length > 0) {
    // With top the held layer, nothing over it and no stray focus, the two
    // walks keep the same elements, so the inert walk stands for both.
    const stray = strayFocus(top);
    const silenced = top === held && !over.length && !stray.length
      ? inert
      : covered(document.body, [...stray, ...top.regions, ...live, ...openedFrom([top])], DOM, quiet);
    for (const el of silenced) {
      if (el.hasAttribute(HIDDEN) || el.getAttribute("aria-hidden") !== "true") hidden.add(el);
    }
  }
  for (const el of Array.from(document.querySelectorAll("[" + HIDDEN + "]"))) {
    if (hidden.has(el)) continue;
    const prior = el.getAttribute(HIDDEN);
    el.removeAttribute(HIDDEN);
    if (el.getAttribute("aria-hidden") !== "true") continue;
    if (prior) el.setAttribute("aria-hidden", prior);
    else el.removeAttribute("aria-hidden");
  }
  for (const el of hidden) {
    const own = el.getAttribute("aria-hidden");
    if (own === "true") continue;
    el.setAttribute(HIDDEN, own ?? "");
    ariaWritten.add(el);
    el.setAttribute("aria-hidden", "true");
  }
  reconcile("inert", INERT, Array.from(document.querySelectorAll("[" + INERT + "]")), inert, attributes);
}
