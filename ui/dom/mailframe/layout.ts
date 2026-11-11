import { place } from "./rules";

const READER = ".gm-reader";
export const READER_BODY = ".gm-reader-body";
export const WRAP = "[data-slot=mail-frame]";
export const VIEW = "[data-slot=mail-frame-view]";
export const SIZE = "[data-action=mail-frame-size]";

type Insets = { left: number; right: number; bottom: number };

// insetsOf reads how far a view bleeds out of the padding around it. The view's
// negative margins are the --mail-frame-inset-* variables; the browser resolves
// a margin to its used length in px, whatever var(), calc() or rem it was
// written with, which a custom property read straight back would not.
function insetsOf(view: HTMLElement): Insets {
  const cs = getComputedStyle(view);
  const px = (v: string) => Math.max(0, -(parseFloat(v) || 0));
  return { left: px(cs.marginLeft), right: px(cs.marginRight), bottom: px(cs.marginBottom) };
}

// pad gives the email's root element the insets as padding, divided by the
// scale the frame is shown at so they keep their size on screen. It goes on
// the root rather than body so a sender's own body padding and margins stay as
// they are; inline and !important so a sender's html{padding:0!important}
// cannot take it away; and border-box so a sender's html{width:100%} still
// fits the frame. A view with no insets leaves the root to the sender, once
// any padding pad gave it is gone.
const padded = new WeakSet<HTMLElement>();

function pad(root: HTMLElement, insets: Insets, scale: number): void {
  const props = ["padding-left", "padding-right", "padding-bottom", "box-sizing"];
  if (insets.left <= 0 && insets.right <= 0 && insets.bottom <= 0) {
    if (padded.delete(root)) props.forEach((p) => root.style.removeProperty(p));
    return;
  }
  padded.add(root);
  root.style.setProperty("padding-left", insets.left / scale + "px", "important");
  root.style.setProperty("padding-right", insets.right / scale + "px", "important");
  root.style.setProperty("padding-bottom", insets.bottom / scale + "px", "important");
  root.style.setProperty("box-sizing", "border-box", "important");
}

// contentWidth is how wide the email's content is, without the left and right
// padding pad gave the root. Body's own scrollWidth counts everything inside
// it from its left edge, with none of the root's padding, so it measures the
// same in every engine; where body starts is added for a sender's body margin.
// The root's scrollWidth catches anything outside body, less the padding.
function contentWidth(doc: Document, left: number, right: number): number {
  const root = doc.documentElement;
  const body = doc.body;
  const bodyLeft = body.getBoundingClientRect().left + (doc.defaultView?.scrollX ?? 0) - left;
  return Math.max(root.scrollWidth - left - right, bodyLeft + body.scrollWidth);
}

// matchSelection gives the email's document the page's selection colors, as
// they resolve for the current theme.
const SELECTION: [string, string][] = [
  ["--selection", "--mail-frame-selection"],
  ["--selection-foreground", "--mail-frame-selection-foreground"],
];

export function matchSelection(doc: Document): void {
  const page = getComputedStyle(document.documentElement);
  for (const [from, to] of SELECTION) {
    const value = page.getPropertyValue(from).trim();
    if (value) doc.documentElement.style.setProperty(to, value);
    else doc.documentElement.style.removeProperty(to);
  }
}

function heightOf(frame: HTMLIFrameElement, doc: Document): number {
  const root = doc.documentElement;
  const body = doc.body;
  if (frame.closest(READER)) return Math.max(body.scrollHeight, root.scrollHeight);
  return Math.max(80, body.scrollHeight + 24, body.offsetHeight + 24, root.scrollHeight + 24, root.offsetHeight + 24);
}

// gutterOf is how much of a reader's width is not its content box's: its
// scrollbar, when it has one, and its borders, which do not change.
export function gutterOf(reader: HTMLElement | null): number {
  return reader ? reader.offsetWidth - reader.clientWidth : 0;
}

// size lays a frame's document out and sizes the frame to it. In a view, a
// document wider than the view's room for it (its width less the insets) is
// widened to its content and scaled to fit, unless the reader chose full size.
// The insets are padding on the document, so the frame is the content's width
// plus the insets at the scale it is shown at, and fills the view exactly. The
// height is measured with the frame collapsed so a shrinking body (e.g. a
// re-hidden quote) isn't pinned to the frame's old height by
// documentElement.scrollHeight. The view (not the wrapper around it) clips and
// carries the scaled height, so the wrapper's button -- in normal flow above
// the view -- is never clipped or scaled with the message; data-scaled and the
// button's state still live on the wrapper, since that is what a reader or
// script looks for.
export function size(
  frame: HTMLIFrameElement,
  doc: Document,
): { height: number; viewWidth: number | undefined } | undefined {
  const root = doc.documentElement;
  const view = frame.closest<HTMLElement>(VIEW);
  const wrap = frame.closest<HTMLElement>(WRAP);
  // A frame inside a closed collapsible measures to a zero-width view; leave
  // it alone and let the collapsible's own open handling re-fit it.
  if (view?.clientWidth === 0) return undefined;
  const viewWidth = view?.clientWidth;
  const full = !!wrap && wrap.hasAttribute("data-full-size");
  let scale = 1;
  if (view) {
    const insets = insetsOf(view);
    const sides = insets.left + insets.right;
    // In full size, measuring widens the frame to the available width, which
    // leaves the view with no horizontal overflow and the browser clamps its
    // scrollLeft to 0. Save it and restore it after, since the browser clamps
    // it back into whatever range the new content allows.
    const scrollLeft = full ? view.scrollLeft : 0;
    // The mobile stylesheet caps the frame at 100%, which would stop it widening.
    frame.style.maxWidth = "none";
    frame.style.transform = "";
    const available = view.clientWidth - sides;
    const fitted = place(available, sides, full, (w, at) => {
      pad(root, insets, at);
      frame.style.width = w + sides / at + "px";
      // A sender's own overflow on body (or an element inside it) would
      // otherwise hide that overflow from the root, so both are measured.
      return contentWidth(doc, insets.left / at, insets.right / at);
    });
    scale = fitted.scale;
    const shown = full ? 1 : scale;
    pad(root, insets, shown);
    const widened = fitted.width > available;
    frame.style.width = widened ? fitted.width + sides / shown + "px" : "";
    frame.style.maxWidth = widened ? "none" : "";
    if (full) view.scrollLeft = scrollLeft;
  }
  const s = full ? 1 : scale;
  frame.style.height = "0px";
  const height = heightOf(frame, doc);
  frame.style.height = height + "px";
  frame.style.transformOrigin = s < 1 ? "0 0" : "";
  frame.style.transform = s < 1 ? "scale(" + s + ")" : "";
  // A frame without a view (and so without a wrapper either) stays height-only.
  if (!view) return { height, viewWidth };
  view.style.height = s < 1 ? Math.ceil(height * s) + "px" : "";
  // The clipped view still has the unscaled frame as scrollable overflow, so
  // nothing (e.g. focus) may leave it scrolled.
  if (s < 1) view.scrollLeft = 0;
  if (!wrap) return { height, viewWidth };
  wrap.toggleAttribute("data-scaled", scale < 1);
  const button = wrap.querySelector<HTMLButtonElement>(SIZE);
  if (button) {
    button.hidden = scale >= 1;
    button.textContent = full ? "Fit to width" : "View full size";
  }
  return { height, viewWidth };
}
