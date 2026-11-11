// MIN_SCALE is the smallest a message is shrunk to; one wider still scrolls
// sideways at this scale. It is low enough to be effectively off.
export const MIN_SCALE = 0.05;
// MAX_WIDEN bounds how many times widening may cross a sender's breakpoint
// into a wider layout.
export const MAX_WIDEN = 3;

export type Fit = { scale: number; width: number };

// settle finds the width a document lays out at and the scale that fits that
// width into available. measure lays the document out at a width and returns
// its scrollWidth. Content that fits at available is left there; otherwise the
// document is widened to its content, re-measured in case that crossed one of
// its breakpoints, and scaled.
export function settle(available: number, measure: (width: number) => number): Fit {
  let content = measure(available);
  if (content <= available + 1) return { scale: 1, width: available };
  let width = available;
  for (let i = 0; i < MAX_WIDEN && content > width + 1; i++) {
    width = content;
    content = measure(width);
  }
  width = Math.max(width, content);
  return { scale: Math.max(available / width, MIN_SCALE), width };
}

// place is settle for a frame whose document carries insets of sides px in
// all as padding. The frame is the content's width plus the insets at the
// scale it is shown at, so shrinking it widens the frame -- and the email's
// media queries, which see the frame -- after it was measured. measure lays
// the document out at a content width with the insets at a scale. If the
// content at the shown scale is wider than measured (a breakpoint in that
// jump), the document is settled again from there, once. Full size shows the
// insets at their own size.
export function place(
  available: number,
  sides: number,
  full: boolean,
  measure: (width: number, scale: number) => number,
): Fit {
  let fitted = settle(available, (w) => measure(w, 1));
  const shown = full ? 1 : fitted.scale;
  if (shown < 1 && sides > 0) {
    const content = measure(fitted.width, shown);
    if (content > fitted.width + 1) fitted = settle(available, (w) => measure(Math.max(w, content), shown));
  }
  return fitted;
}

// unchanged says whether a size an observed element reports is the one fit
// last left it at (fitted), to within a pixel. A root or body that is
// height:100% takes the height fit gave the frame, and a view is the width fit
// measured it at, so a fit for either would only find the same frame again.
// Before fit has run, nothing is unchanged.
export function unchanged(measured: number, fitted: number | undefined): boolean {
  return fitted !== undefined && Math.abs(measured - fitted) < 1;
}

// Fitted is what one fit of a frame measured and left: the width of the view
// it measured at, how much of the reader's width went to its scrollbar then,
// whether it widened the frame to a fixed width to shrink it, and the height
// it left in the reader.
export type Fitted = { fixed: boolean; gutter: number; height: number; width: number };

// Hold keeps a frame at one fit while its view's width stays between narrow
// and wide and the window has resized no more times than resizes.
export type Hold = { narrow: number; resizes: number; wide: number };

// Seen is the view's width now, the reader's scrollbar's share of the
// reader's width now, and how many times the window has resized.
export type Seen = { gutter: number; resizes: number; width: number };

// refitOnWidth decides whether a frame whose view changed width fits again,
// given its last two fits (oldest first) and any hold it is under. A fit can
// add or take away the reader's own scrollbar where scrollbars take width,
// which narrows or widens the view. An email shown shorter at a narrower
// width -- shrunk to fit, its scale following the view -- and on the edge of
// the reader's height fits taller, brings the scrollbar, fits shorter, loses
// it, and so on every animation frame. A view going back to the width of the
// fit before last, with the scrollbar flipped since the last fit, is that
// loop. The frame then keeps the one fit that shows the whole email at both
// widths, and holds it until the view leaves those widths or the window
// resizes:
//   - the narrower fit when it is shrunk to a fixed width, which the wider
//     view also holds;
//   - otherwise the taller, which an email that is shorter narrower still
//     fits at the narrower width.
// Keeping the fit before last means fitting again, since the view is back at
// its width; keeping the last means leaving the frame as it is.
export function refitOnWidth(
  seen: Seen,
  recent: readonly Fitted[],
  hold: Hold | undefined,
): { fit: boolean; hold?: Hold } {
  if (hold && hold.resizes === seen.resizes && seen.width > hold.narrow - 1 && seen.width < hold.wide + 1) {
    return { fit: false, hold };
  }
  const last = recent.at(-1);
  const before = recent.at(-2);
  if (!last || !before) return { fit: true };
  const back = Math.abs(seen.width - before.width) < 1 && Math.abs(seen.gutter - last.gutter) >= 1;
  if (!back) return { fit: true };
  const [narrower, wider] = before.width < last.width ? [before, last] : [last, before];
  const taller = before.height >= last.height ? before : last;
  const keep = narrower.fixed ? narrower : taller;
  return { fit: keep === before, hold: { narrow: narrower.width, resizes: seen.resizes, wide: wider.width } };
}
