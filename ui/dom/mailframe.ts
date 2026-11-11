import { readFrameDocument, reported } from "./failures";
import { prepareDocument } from "./mailframe/document";
import { gutterOf, matchSelection, READER_BODY, SIZE, size, VIEW, WRAP } from "./mailframe/layout";
import { type Fitted, type Hold, refitOnWidth, unchanged } from "./mailframe/rules";

// Coordinates sandboxed email frames: discovery, resize batching, scroll
// preservation and readiness. Layout, sizing rules and document preparation
// live in mailframe/. See docs/mail-rendering.md for the rendering contract.

const FRAME = "iframe.gm-msg-body";
const COLLAPSIBLE_STATE = "data-tui-collapsible-state";

// The browser's own scroll anchoring keeps the text in view still when
// something above it changes height. Where there is none (Safari), fit does it
// for a frame wholly above the reader's visible top. Old engines have no
// CSS.supports, which TypeScript's DOM types leave out.
const anchorsScroll = typeof CSS !== "undefined" && typeof CSS.supports === "function"
  && CSS.supports("overflow-anchor", "auto");

// fittedHeight is the height fit last gave each frame, and fittedViewWidth
// the width of the frame's view when fit measured it.
const fittedHeight = new WeakMap<HTMLIFrameElement, number>();
const fittedViewWidth = new WeakMap<HTMLIFrameElement, number>();
const fittedContentWidth = new WeakMap<Element, number>();

// recentFits is each frame's last two fits, oldest first, and holds the hold
// refitOnWidth put it under, which any fit ends.
const recentFits = new WeakMap<HTMLIFrameElement, Fitted[]>();
const holds = new WeakMap<HTMLIFrameElement, Hold>();
let resizes = 0;

// fit sizes a frame to its document. Measuring lays the reader out with the
// frame at 0, which clamps the reader's scroll offset to the shorter content,
// and putting the height back does not undo that. So the reader's offset is put
// back afterwards, moved by the change in height when the frame is above what
// is in view and the browser does not anchor. A browser that anchors has
// already moved it by that change for a frame above, and writing the old offset
// back would undo that, so such a frame is left to it. It is only written when
// it has to change, as writing it stops a momentum scroll in progress. In a
// wrapper, the wrapper is what the reader sees, so it is what is measured.
function fit(frame: HTMLIFrameElement): void {
  const doc = readFrameDocument(frame);
  // A document still parsing has no body yet, which TypeScript's DOM types leave out.
  if (!doc || !(doc.body as HTMLElement | null)) return;
  reported({ operation: "mailframe.fit" }, () => fitDocument(frame, doc), undefined);
}

function fitDocument(frame: HTMLIFrameElement, doc: Document): void {
  matchSelection(doc);
  const box = frame.closest<HTMLElement>(WRAP) ?? frame;
  const reader = frame.closest<HTMLElement>(READER_BODY);
  const top = reader ? reader.scrollTop : 0;
  const before = box.offsetHeight;
  const above = !!reader
    && box.getBoundingClientRect().bottom
      <= reader.getBoundingClientRect().top + (parseFloat(getComputedStyle(reader).scrollPaddingTop) || 0);
  const width = (frame.closest<HTMLElement>(VIEW) ?? frame).clientWidth;
  const gutter = gutterOf(reader);
  holds.delete(frame);
  const measured = size(frame, doc);
  if (measured) {
    fittedHeight.set(frame, measured.height);
    if (measured.viewWidth !== undefined) fittedViewWidth.set(frame, measured.viewWidth);
  }
  // The frame's elements are its window's: an SVG one has no offsetWidth.
  const html = doc.defaultView?.HTMLElement;
  for (const target of observed.get(frame) ?? []) {
    if (target.ownerDocument !== doc) continue;
    fittedContentWidth.set(
      target,
      html && target instanceof html ? target.offsetWidth : target.getBoundingClientRect().width,
    );
  }
  if (width > 0) {
    const fitted = { fixed: frame.style.width !== "", gutter, height: box.offsetHeight, width };
    recentFits.set(frame, [...(recentFits.get(frame) ?? []).slice(-1), fitted]);
  }
  if (!reader || (above && anchorsScroll)) return;
  const want = top + (above ? box.offsetHeight - before : 0);
  if (reader.scrollTop !== want && (above || reader.scrollTop < top)) reader.scrollTop = want;
}

function fitAll(): void {
  document.querySelectorAll<HTMLIFrameElement>(FRAME).forEach(fit);
}

// One ResizeObserver refits every frame when the frame's email or its view
// changes size: an image or a web font arriving, a sender's layout settling,
// or the reader's width changing. That last one also covers a frame widened
// to a fixed width to be shrunk, whose own document does not resize.
//
// It watches four things:
//   - each document's root;
//   - its body;
//   - the body's children, since an email whose html and body are
//     height:100% pins both to the frame and only what is inside them
//     grows;
//   - the view.
//
// Content can also grow inside an element of fixed height -- a wrapper that is
// height:100% inside an html and body that are too -- where none of those
// resize. What grows an email after it is set up is something arriving or the
// reader opening something in it, so every load or error inside its document
// (an image, a nested frame, a stylesheet), every <details> toggled and every
// input changed (a checkbox can decide what an email's CSS shows), heard in
// the capture phase because load, error and toggle do not bubble, and every
// web font that finishes loading queues a fit as well.
//
// A load or a new srcdoc replaces the document, so setup observes each
// document it sets up, and forget lets each go as its page hides. The
// observer is the page's own, because a frame sandboxed without allow-scripts
// never runs a callback made in its window; event listeners made in the page
// run there.
//
// A fit waits for the next animation frame, once per frame however many of
// its elements resized. That way the resizing a fit causes is seen in the
// following frame rather than inside the callback, where it would be a
// ResizeObserver loop. A root or body at the height fit gave the frame, and a
// view at the width fit measured, are only following the last fit, so they
// do not fit again. Content widths are recorded after each fit, so widths
// changed by fitting do not queue another fit, while independent width
// changes still fit even when the content's height stays fixed. A frame that
// only its view's width asked to fit is left to refitOnWidth, which holds it
// still while the reader's scrollbar would otherwise flip with every fit.
// Anything else that asks fits it, and so ends the hold.
let sizes: ResizeObserver | null = null;
const frameOf = new WeakMap<Element, HTMLIFrameElement>();
const observed = new WeakMap<HTMLIFrameElement, Element[]>();
const listened = new WeakSet<Document>();
const reportedHeight = new WeakMap<Element, number>();
// due maps each frame queued to whether only its view's width asked for it.
const due = new Map<HTMLIFrameElement, boolean>();
let flushing = 0;

function fitNextFrame(frame: HTMLIFrameElement, widthOnly = false): void {
  due.set(frame, (due.get(frame) ?? true) && widthOnly);
  if (!flushing) flushing = requestAnimationFrame(flush);
}

function fitOnWidth(frame: HTMLIFrameElement): void {
  const seen = {
    gutter: gutterOf(frame.closest<HTMLElement>(READER_BODY)),
    resizes,
    width: (frame.closest<HTMLElement>(VIEW) ?? frame).clientWidth,
  };
  const step = refitOnWidth(seen, recentFits.get(frame) ?? [], holds.get(frame));
  if (step.fit) fit(frame);
  if (step.hold) holds.set(frame, step.hold);
}

// flush fits every frame due and only then announces them, since due is
// cleared first and the first announcement's sized() already counts every
// frame in the batch as fitted.
function flush(): void {
  flushing = 0;
  const batch = Array.from(due).filter(([frame]) => frame.isConnected);
  due.clear();
  for (const [frame, widthOnly] of batch) {
    if (widthOnly) fitOnWidth(frame);
    else fit(frame);
  }
  for (const [frame] of batch) announce(frame);
}

function resized(entries: ResizeObserverEntry[]): void {
  for (const entry of entries) {
    const target = entry.target;
    const frame = frameOf.get(target);
    if (!frame) continue;
    if (target === frame.closest(VIEW)) {
      if (!unchanged(entry.contentRect.width, fittedViewWidth.get(frame))) fitNextFrame(frame, true);
      continue;
    }
    // Safari before 15.4 has no borderBoxSize, which TypeScript's DOM types leave out.
    const box = (entry.borderBoxSize as readonly ResizeObserverSize[] | undefined)?.[0];
    const height = box ? box.blockSize : entry.contentRect.height;
    const width = box ? box.inlineSize : entry.contentRect.width;
    const widthChanged = !unchanged(width, fittedContentWidth.get(target));
    const reported = reportedHeight.get(target);
    reportedHeight.set(target, height);
    if (unchanged(height, reported) && !widthChanged) continue;
    if (target === target.ownerDocument.documentElement || target === target.ownerDocument.body) {
      if (unchanged(height, fittedHeight.get(frame)) && !widthChanged) continue;
    }
    fitNextFrame(frame);
  }
}

function observe(frame: HTMLIFrameElement, doc: Document): void {
  sizes ??= new ResizeObserver(resized);
  const view = frame.closest<HTMLElement>(VIEW);
  const targets: Element[] = [doc.documentElement, doc.body, ...Array.from(doc.body.children)];
  if (view) targets.push(view);
  const before = new Set(observed.get(frame));
  const wanted = new Set(targets);
  for (const target of before) if (!wanted.has(target)) sizes.unobserve(target);
  for (const target of targets) {
    if (before.has(target)) continue;
    frameOf.set(target, frame);
    sizes.observe(target);
  }
  observed.set(frame, targets);
  if (listened.has(doc)) return;
  listened.add(doc);
  const arrived = () => fitNextFrame(frame);
  for (const type of ["load", "error", "toggle", "change"]) doc.addEventListener(type, arrived, true);
  // Old engines have no document.fonts, which TypeScript's DOM types leave out.
  (doc.fonts as FontFaceSet | undefined)?.addEventListener("loadingdone", arrived);
  doc.defaultView?.addEventListener("pagehide", (e) => {
    if (!e.persisted) forget(frame, doc);
  });
}

// forget stops observing a frame's document once it is gone: a new srcdoc
// replaced it, or the frame left the page or moved within it. Chrome keeps
// measuring a gone document's elements without delivering them, which ends
// every frame in a ResizeObserver loop error, and setup only observes the
// next document once it loads. A page kept in the back/forward cache keeps
// its documents, so they stay observed.
function forget(frame: HTMLIFrameElement, doc: Document): void {
  const kept: Element[] = [];
  for (const target of observed.get(frame) ?? []) {
    if (target.ownerDocument === doc) sizes?.unobserve(target);
    else kept.push(target);
  }
  observed.set(frame, kept);
}

// unobserve lets a frame's view go once the frame has left the page; forget
// has already let its document go. A frame moved within the page is
// connected again by the time the mutation is seen, and keeps its view
// observed.
function unobserve(frame: HTMLIFrameElement): void {
  if (frame.isConnected) return;
  for (const target of observed.get(frame) ?? []) sizes?.unobserve(target);
  observed.delete(frame);
  due.delete(frame);
}

// SIZED is dispatched, bubbling, on a frame each time setup or the observer
// fits it, and sized says whether the fitting under a root is over. A frame
// is sized once all of these hold:
//   - the document it shows is the one setup set up;
//   - that document has finished loading;
//   - no fit is queued for it;
//   - every image in it has loaded or failed;
//   - no web font in it is still loading, since the load event does not
//     wait for one that layout asks for late, and its loadingdone queues a
//     fit and so a fresh announcement.
// Then nothing that is coming will resize it. A frame in a closed message
// takes no room and is not counted. Scroll memory ends its hold on it
// (scrollmemory.ts).
export const SIZED = "mailframe:sized";

const ready = new WeakMap<HTMLIFrameElement, Document>();

function announce(frame: HTMLIFrameElement): void {
  frame.dispatchEvent(new Event(SIZED, { bubbles: true }));
}

export function sized(root: ParentNode): boolean {
  for (const frame of Array.from(root.querySelectorAll<HTMLIFrameElement>(FRAME))) {
    if (frame.getClientRects().length === 0) continue;
    if (due.has(frame)) return false;
    let doc: Document | null;
    try {
      doc = frame.contentDocument;
    } catch {
      continue;
    }
    if (!doc || ready.get(frame) !== doc || doc.readyState !== "complete") return false;
    if (Array.from(doc.images).some((img) => !img.complete)) return false;
    if ((doc.fonts as FontFaceSet | undefined)?.status === "loading") return false;
  }
  return true;
}

// setup prepares a frame's document and fits it: links open in a new tab,
// quoted reply text goes behind a toggle, and the observer watches the
// document so whatever later resizes it -- an image loading, a font arriving
// -- fits the frame again. It is safe to run again on the same document.
function setup(frame: HTMLIFrameElement): void {
  let doc: Document | null;
  try {
    doc = frame.contentDocument;
  } catch {
    return;
  }
  if (!doc || !(doc.body as HTMLElement | null)) return;
  prepareDocument(doc, () => fit(frame));
  observe(frame, doc);
  fit(frame);
  ready.set(frame, doc);
  announce(frame);
}

// watch sets a frame up once its document is ready, and again on every load.
// A srcdoc iframe may finish loading before a load listener attaches, so it
// polls for the document too.
function watch(frame: HTMLIFrameElement): void {
  if (frame.dataset.frameWatched === "true") return;
  frame.dataset.frameWatched = "true";
  let tries = 0;
  const poll = setInterval(() => {
    tries++;
    // A frame whose document is out of reach is not ready.
    const ready = !!(readFrameDocument(frame)?.body as HTMLElement | null | undefined)?.innerHTML.trim();
    if (ready) {
      setup(frame);
      clearInterval(poll);
    } else if (tries > 60) {
      clearInterval(poll);
    }
  }, 50);
  frame.addEventListener("load", () => setup(frame));
}

function watchWithin(root: ParentNode): void {
  root.querySelectorAll<HTMLIFrameElement>(FRAME).forEach(watch);
}

declare global {
  interface Window {
    HousecatMailFrame: { fit: typeof fit; fitAll: typeof fitAll; sized: typeof sized };
  }
}

if (typeof document !== "undefined") {
  window.HousecatMailFrame = { fit, fitAll, sized };
  window.addEventListener("resize", () => resizes++);
  watchWithin(document);
  // An iframe measures to zero while it is display:none, so anything revealed
  // is re-fitted once it is visible; and a frame added anywhere is watched.
  new MutationObserver((records) => {
    for (const r of records) {
      if (r.type === "attributes") {
        const el = r.target;
        if (el instanceof Element && el.getAttribute(COLLAPSIBLE_STATE) === "open") {
          el.querySelectorAll<HTMLIFrameElement>(FRAME).forEach(fit);
        }
        continue;
      }
      r.addedNodes.forEach((n) => {
        if (!(n instanceof Element)) return;
        if (n instanceof HTMLIFrameElement && n.matches(FRAME)) watch(n);
        else watchWithin(n);
      });
      r.removedNodes.forEach((n) => {
        if (!(n instanceof Element)) return;
        if (n instanceof HTMLIFrameElement && n.matches(FRAME)) unobserve(n);
        else n.querySelectorAll<HTMLIFrameElement>(FRAME).forEach(unobserve);
      });
    }
  }).observe(document.documentElement, { attributeFilter: [COLLAPSIBLE_STATE], childList: true, subtree: true });
  document.addEventListener(
    "toggle",
    (e) => {
      const t = e.target;
      if (t instanceof HTMLDetailsElement && t.open) t.querySelectorAll<HTMLIFrameElement>(FRAME).forEach(fit);
    },
    true,
  );
  document.addEventListener("click", (e) => {
    const button = e.target instanceof Element ? e.target.closest(SIZE) : null;
    const wrap = button?.closest<HTMLElement>(WRAP);
    const frame = wrap?.querySelector<HTMLIFrameElement>(FRAME);
    if (!wrap || !frame) return;
    wrap.toggleAttribute("data-full-size");
    fit(frame);
  });
  // The theme switches with a class on the root; a frame's selection colors
  // follow it.
  new MutationObserver(() => {
    document.querySelectorAll<HTMLIFrameElement>(FRAME).forEach((frame) => {
      const doc = readFrameDocument(frame);
      if (doc?.documentElement) matchSelection(doc);
    });
  }).observe(document.documentElement, { attributeFilter: ["class"] });
}
