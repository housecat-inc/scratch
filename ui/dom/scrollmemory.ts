// Scroll memory keeps where the mail list and the reader were scrolled, so
// Back puts them there again. Each scroller carries data-list-key, the page
// it shows as the server names it (uimail.ListKey); its memory is one
// sessionStorage slot, mail:list:<key>, holding the most recent departure:
// the offset and the row the user was on. A departure saves it -- a content
// navigation leaving the page (the lane's depart hook) or the document going
// away (pagehide) -- and an arrival reads it: a returning navigation, a
// history traversal or the reader's Back, puts the scroller back; any other
// navigation to the page forgets it and lands at the top, as a fresh visit
// would. A full load is a return only when the browser says so -- a reload
// or a back or forward traversal -- and restores then; a new tab, a typed
// URL or a link the browser loads lands at the top like any fresh visit.
//
// Nothing here reaches the document outside install(), so Vitest runs the
// rules in Node.

import * as Schema from "effect/Schema";

import { type Nav, navOf, on } from "./content";
import { forgetStorage, readJSON, readStorage, writeStorage } from "./failures";
import { SIZED, sized } from "./mailframe";

export const PREFIX = "mail:list:";

// SCROLLER is what scrolls a mail page: the list, or the reader.
export const SCROLLER = "[data-slot=mail-list], #readerBody";

export type Memory = { rowID: string; scrollTop: number };

// An arrival restores the page's memory, forgets it, or keeps it as it is.
export type Arrival = "forget" | "keep" | "restore";

export function storageKey(listKey: string): string {
  return PREFIX + listKey;
}

// Saved is a slot as save writes it: the offset and, when it has one, the row
// the user was on.
const Saved = Schema.Struct({ rowID: Schema.optionalKey(Schema.String), scrollTop: Schema.Finite });
const isSaved = Schema.is(Saved);

// parseMemory reads a saved slot; anything save didn't write is no memory at
// all.
export function parseMemory(raw: string | null): Memory | null {
  const saved = readJSON(raw);
  if (!isSaved(saved)) return null;
  return { rowID: saved.rowID ?? "", scrollTop: Math.max(0, saved.scrollTop) };
}

// arrival is what a swap into the content does with the memory of the page
// it brings: a swap the lane did not make (pull to refresh's htmx.swap)
// keeps it, a returning navigation restores it, and any other navigation
// forgets it, which is what keeps a forward visit, Load older among them,
// at the top.
export function arrival(nav: Pick<Nav, "returning"> | null): Arrival {
  if (!nav) return "keep";
  return nav.returning ? "restore" : "forget";
}

// scrollerOf is the one place that maps a page to the element that scrolls
// it; a page-scroll rework changes it here.
export function scrollerOf(view: ParentNode): HTMLElement | null {
  return view.querySelector<HTMLElement>(SCROLLER);
}

const ROW = "[data-slot=mail-row]";

function keyOf(scroller: HTMLElement): string {
  return scroller.getAttribute("data-list-key") ?? "";
}

function read(key: string): Memory | null {
  return parseMemory(readStorage("session", storageKey(key)));
}

// Storage that is unavailable or full remembers nothing, and the list opens
// at its top.
function write(key: string, memory: Memory): void {
  writeStorage("session", storageKey(key), JSON.stringify(memory));
}

function forget(key: string): void {
  forgetStorage("session", storageKey(key));
}

function contentScroller(): HTMLElement | null {
  const content = document.querySelector("[data-mail-content]");
  return content ? scrollerOf(content) : null;
}

// save records where scroller stands and the row the user was on: the row
// from belongs to, when a row's link is what departs, else the list's
// current row. The collection marks its current row on the wrapper around a
// mail row (uikit MailRow), so the id is read from the mail row inside it.
// A scroller still held stands where the hold puts it, not where its
// unfitted content clamps it. A page left at its top on no row is where any
// arrival lands it, so it keeps no slot: only pages left somewhere take
// room in sessionStorage.
function save(scroller: HTMLElement, from: Element | null): void {
  const key = keyOf(scroller);
  if (!key) return;
  const own = from?.closest<HTMLElement>(ROW);
  const row = own && scroller.contains(own)
    ? own
    : window.HousecatCollection.current(scroller)?.querySelector<HTMLElement>(ROW) ?? null;
  const rowID = row?.getAttribute("data-id") ?? "";
  const scrollTop = held?.scroller === scroller ? held.top : scroller.scrollTop;
  if (scrollTop <= 0 && !rowID) forget(key);
  else write(key, { rowID, scrollTop });
}

// inView is whether row shows whole within the scroller's scroll padding.
function inView(scroller: HTMLElement, row: HTMLElement): boolean {
  const box = row.getBoundingClientRect();
  const view = scroller.getBoundingClientRect();
  const style = getComputedStyle(scroller);
  const over = parseFloat(style.scrollPaddingTop) || 0;
  const under = parseFloat(style.scrollPaddingBottom) || 0;
  return box.top >= view.top + over && box.bottom <= view.bottom - under;
}

// HOLD_MS is the longest an arrival keeps its offset while the page grows
// under it.
export const HOLD_MS = 2000;

let release: (() => void) | null = null;

// held is the scroller a hold keeps and the offset it keeps it at, while
// the hold lasts.
let held: { scroller: HTMLElement; top: number } | null = null;

// hold puts scroller at top and keeps it there while what it holds grows.
// The offset was saved with a reader's message frames fitted. They are
// fitted only after they load (mailframe.ts), so until then the reader is
// short and the offset clamps. Fit's own compensation, like the browser's
// scroll anchoring, would keep what the unfitted page showed rather than the
// offset. So each resize of the scroller's content writes the offset again,
// after layout and before paint, and neither of them wins.
//
// Once every frame in the scroller is sized (mailframe.ts says so with
// SIZED), nothing more is coming from them, so the hold writes the offset a
// last time and ends -- unless the offset still clamps, which means the
// scroller is short of content that is still coming from elsewhere, and the
// hold keeps it. Otherwise it ends when the user scrolls (a wheel, a touch,
// a press or a key), at the next departure, or after HOLD_MS.
function hold(scroller: HTMLElement, top: number): void {
  release?.();
  const inputs = ["keydown", "pointerdown", "touchstart", "wheel"] as const;
  const sizes = new ResizeObserver(() => {
    if (scroller.isConnected) scroller.scrollTop = top;
    else end();
  });
  const timer = setTimeout(end, HOLD_MS);
  const state = { scroller, top };
  const settle = () => {
    if (!sized(scroller)) return;
    scroller.scrollTop = top;
    if (Math.abs(scroller.scrollTop - top) <= 1) end();
  };
  function end(): void {
    sizes.disconnect();
    clearTimeout(timer);
    scroller.removeEventListener(SIZED, settle);
    for (const type of inputs) document.removeEventListener(type, end, true);
    if (release === end) release = null;
    if (held === state) held = null;
  }
  scroller.scrollTop = top;
  for (const child of Array.from(scroller.children)) sizes.observe(child);
  for (const type of inputs) document.addEventListener(type, end, { capture: true, passive: true });
  scroller.addEventListener(SIZED, settle);
  release = end;
  held = state;
}

// restore puts scroller back where memory left it, held there while its
// content settles, and makes the row the user was on the list's current
// one, scrolling it to the middle -- and holding that -- only when the
// offset leaves it out of view. Focus follows only when the list already
// holds it: the key router's recovery lands focus on the list when the swap
// took the focused element (ui/dom/keyfocus.ts), and the row the user left
// is where it belongs, not the list's first row.
function restore(scroller: HTMLElement, memory: Memory): void {
  let top = memory.scrollTop;
  scroller.scrollTop = top;
  const row = memory.rowID
    ? scroller.querySelector<HTMLElement>(`${ROW}[data-id="${CSS.escape(memory.rowID)}"]`)
    : null;
  if (row) {
    const focus = row.closest('[data-collection="root"]')?.contains(document.activeElement) === true;
    window.HousecatCollection.select(row, { focus, scroll: false });
    if (!inView(scroller, row)) {
      row.scrollIntoView({ block: "center" });
      top = scroller.scrollTop;
    }
  }
  hold(scroller, top);
}

// returnsOnLoad says whether a full load, by the browser's navigation type,
// is the user coming back to a page rather than arriving at it.
export function returnsOnLoad(type: string | undefined): boolean {
  return type === "reload" || type === "back_forward";
}

function install(): void {
  on("depart", (nav: Nav) => {
    const scroller = contentScroller();
    if (scroller) save(scroller, nav.ctx.sourceElement);
    release?.();
  });

  window.addEventListener("pagehide", () => {
    const scroller = contentScroller();
    if (scroller) save(scroller, null);
  });

  // The arrival runs as the swap lands, before the browser paints it, so a
  // returning page never shows at the top first. Every swap into the
  // content has its head seated, pull to refresh's included; only the
  // lane's navigations restore or forget.
  document.addEventListener("htmx:after:swap", (e) => {
    const ctx = e.detail?.ctx;
    const root = ctx?.target;
    if (!ctx || !(root instanceof Element) || !root.matches("[data-mail-content]")) return;
    const scroller = scrollerOf(root);
    if (!scroller) return;
    const key = keyOf(scroller);
    if (!key) return;
    const plan = arrival(navOf(ctx));
    if (plan === "forget") {
      forget(key);
      return;
    }
    const memory = plan === "restore" ? read(key) : null;
    if (memory) restore(scroller, memory);
  });

  // A full load the browser reports as a return lands where its page was
  // last left; any other full load forgets the slot and stays at the top.
  const start = () => {
    const scroller = contentScroller();
    const key = scroller ? keyOf(scroller) : "";
    if (!scroller || !key) return;
    const [entry] = performance.getEntriesByType("navigation");
    if (!returnsOnLoad(entry instanceof PerformanceNavigationTiming ? entry.type : undefined)) {
      forget(key);
      return;
    }
    const memory = read(key);
    if (!memory) return;
    restore(scroller, memory);
  };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start, { once: true });
  else start();
}

if (typeof document !== "undefined") install();
