// Anchors a top-layer positioner to an element through Floating UI, which the
// positioner's data attributes configure: it flips the side when the preferred
// one has no room, shifts to stay inside the viewport, points its arrow at the
// anchor, and follows the anchor through scroll and resize. The tooltip,
// popover, menu, listbox and tour scripts all place through here.

import {
  arrow,
  autoUpdate,
  computePosition,
  type ComputePositionConfig,
  type ComputePositionReturn,
  flip,
  offset,
  type Placement,
  shift,
  size,
} from "@floating-ui/dom";
import { intAttr } from "../dom/dom";
import { reportRejection } from "../dom/failures";
import { DEFAULT_SIDE_OFFSET, DEFAULT_SIDE_OFFSET_WITH_ARROW } from "./offsets";

export type Align = "center" | "end" | "start";
export type Side = "bottom" | "left" | "right" | "top";

const ALIGNS: readonly Align[] = ["center", "end", "start"];
const SIDES: readonly Side[] = ["bottom", "left", "right", "top"];

// sideOf and alignOf are value as a side or an alignment, or null for any
// other string.
function sideOf(value: string | null | undefined): Side | null {
  return SIDES.find((side) => side === value) ?? null;
}

function alignOf(value: string | null | undefined): Align | null {
  return ALIGNS.find((align) => align === value) ?? null;
}

const ARROW = "[data-slot$=-arrow]";
const DEFAULT_COLLISION_PADDING = 5;
const MIN_ARROW_PADDING = 18;

// A popup tries the side opposite its own before either perpendicular one, so
// it leaves its axis only when neither side of the anchor has room.
const FALLBACKS: Record<Side, Side[]> = {
  bottom: ["top", "right", "left"],
  left: ["right", "top", "bottom"],
  right: ["left", "top", "bottom"],
  top: ["bottom", "right", "left"],
};

// The side a positioner asked for, kept apart from the data-side it ends up
// with: a flip rewrites the attribute, and the next placement has to start
// from the preference again rather than from where it last landed.
const preferredSides = new WeakMap<Element, Side>();
const tracking = new WeakMap<HTMLElement, () => void>();

// config is the Floating UI placement a positioner's data attributes ask for.
//
// flip checks only the side the popup faces, so a popup that overhangs along
// its alignment still fits there and is shifted in instead; when no side
// fits, it keeps the one it asked for. size sits between two shifts because
// it reads which axes the shift before it clamped: there, the room it
// publishes is what is left on the settled side across the side axis, and the
// whole viewport along the alignment axis, which is the room the popup can
// grow into once the second shift has pulled it inside.
export function config(positioner: HTMLElement): ComputePositionConfig {
  let side = preferredSides.get(positioner);
  if (!side) {
    side = sideOf(positioner.getAttribute("data-side")) ?? "bottom";
    preferredSides.set(positioner, side);
  }
  const align = alignOf(positioner.getAttribute("data-align")) ?? "center";
  const placement = (to: Side): Placement => (align === "center" ? to : `${to}-${align}`);
  const arrowEl = positioner.querySelector<HTMLElement>(ARROW);
  const alignOffset = intAttr(positioner, "data-align-offset", 0);
  const padding = intAttr(positioner, "data-collision-padding", DEFAULT_COLLISION_PADDING);
  return {
    middleware: [
      offset({
        alignmentAxis: alignOffset,
        crossAxis: alignOffset,
        mainAxis: intAttr(
          positioner,
          "data-side-offset",
          arrowEl ? DEFAULT_SIDE_OFFSET_WITH_ARROW : DEFAULT_SIDE_OFFSET,
        ),
      }),
      flip({
        crossAxis: false,
        fallbackPlacements: FALLBACKS[side].map(placement),
        fallbackStrategy: "initialPlacement",
        padding,
      }),
      shift({ padding }),
      size({
        apply: ({ availableHeight, availableWidth, elements, rects }) => {
          const style = elements.floating.style;
          style.setProperty("--available-width", Math.max(0, availableWidth) + "px");
          style.setProperty("--available-height", Math.max(0, availableHeight) + "px");
          style.setProperty("--positioner-width", rects.floating.width + "px");
          style.setProperty("--positioner-height", rects.floating.height + "px");
        },
        padding,
      }),
      shift({ crossAxis: true, padding }),
      arrowEl
      && arrow({
        element: arrowEl,
        padding: Math.max(MIN_ARROW_PADDING, intAttr(positioner, "data-arrow-padding", MIN_ARROW_PADDING)),
      }),
    ],
    placement: placement(side),
    strategy: "fixed",
  };
}

function isVertical(side: Side): boolean {
  return side === "top" || side === "bottom";
}

function transformOrigin(side: Side, align: Align): string {
  if (isVertical(side)) {
    const x = align === "start" ? "left" : align === "end" ? "right" : "center";
    return x + " " + (side === "top" ? "bottom" : "top");
  }
  const y = align === "start" ? "top" : align === "end" ? "bottom" : "center";
  return (side === "left" ? "right" : "left") + " " + y;
}

// What each tracked positioner was last given, and the popup and arrow it was
// written to. autoUpdate places once as it starts and again as each of its
// observers reports in, and those land where the first did: a placement that
// matches the last one, on the same popup and arrow, writes nothing. untrack
// forgets it, so each opening writes its placement afresh.
const applied = new WeakMap<HTMLElement, { arrow: Element | null; key: string; popup: Element | null }>();

function apply(positioner: HTMLElement, { middlewareData, placement, x, y }: ComputePositionReturn): void {
  const [sideName, alignName] = placement.split("-");
  const side = sideOf(sideName) ?? "bottom";
  const align = alignOf(alignName) ?? "center";
  const popup = positioner.firstElementChild;
  const arrowEl = positioner.querySelector<HTMLElement>(ARROW);
  const arrowAt = arrowEl ? middlewareData.arrow : undefined;
  const key = [x, y, placement, arrowAt?.x, arrowAt?.y].join();
  const last = applied.get(positioner);
  if (last?.key === key && last.popup === popup && last.arrow === arrowEl) return;
  applied.set(positioner, { arrow: arrowEl, key, popup });
  positioner.style.left = x + "px";
  positioner.style.top = y + "px";
  positioner.style.setProperty("--transform-origin", transformOrigin(side, align));
  for (const el of [positioner, popup]) {
    el?.setAttribute("data-side", side);
    el?.setAttribute("data-align", align);
  }
  if (!arrowEl || !arrowAt) return;
  arrowEl.style.left = isVertical(side) ? (arrowAt.x ?? 0) + "px" : "";
  arrowEl.style.top = isVertical(side) ? "" : (arrowAt.y ?? 0) + "px";
  arrowEl.setAttribute("data-side", side);
}

// place positions the positioner against the anchor once. Floating UI resolves
// the position in microtasks, so it lands before the next paint but after the
// code that called place has run on: the promise is for a caller that needs
// the popup where it ends up, to scroll within it, say. It never rejects: a
// placement that fails is reported, and leaves the popup where it was, so a
// caller that has no use for it lets it go with void.
export function place(anchor: Element, positioner: HTMLElement): Promise<void> {
  // The anchor's size goes on before the positioner is measured: a popup may
  // size itself from it -- a menu's minimum width is its trigger's -- and a
  // box measured before it grows would be placed by a width it is about to
  // outgrow, throwing off end alignment.
  const rect = anchor.getBoundingClientRect();
  positioner.style.setProperty("--anchor-width", rect.width + "px");
  positioner.style.setProperty("--anchor-height", rect.height + "px");
  return computePosition(anchor, positioner, config(positioner)).then(
    (at) => apply(positioner, at),
    reportRejection({ operation: "floating.place" }),
  );
}

// track places the positioner now, and again whenever the anchor or the popup
// moves or changes size, until untrack; it answers the first placement, as
// place does. A popup placed again by its own growth matters: a menu that
// shows a field while open grows, and on the side it flipped to that would
// push it over its anchor. Either element leaving the document ends the
// tracking, so a removed anchor never pulls its popup to the corner. Like
// place's, its promise never rejects.
export function track(anchor: Element, positioner: HTMLElement): Promise<void> {
  untrack(positioner);
  let placed = Promise.resolve();
  tracking.set(
    positioner,
    autoUpdate(anchor, positioner, () => {
      if (anchor.isConnected && positioner.isConnected) placed = place(anchor, positioner);
      else untrack(positioner);
    }),
  );
  return placed;
}

export function untrack(positioner: HTMLElement): void {
  tracking.get(positioner)?.();
  tracking.delete(positioner);
  applied.delete(positioner);
}
