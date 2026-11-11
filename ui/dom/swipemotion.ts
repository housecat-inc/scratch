import { intAttr } from "./dom";

const CELL = "[data-swipe-part=action]";
const CHEVRON_WIDTH = 16;
const REVEAL_AT_PX = 8;

export type SwipeMotion = {
  cells: HTMLElement[];
  side: "left" | "right";
  gap: number;
  fullWidth: number;
  margin: number;
  openAt: number;
  revealAt: number;
  size: number;
  width: number;
};

export function measureSwipeMotion(row: HTMLElement, tray: HTMLElement, side: "left" | "right"): SwipeMotion {
  const cells = Array.from(tray.querySelectorAll<HTMLElement>(CELL))
    .filter(cell => cell.closest("[data-swipe-part=tray]") === tray && cell.checkVisibility())
    .sort((a, b) => (Number(getComputedStyle(a).order) || 0) - (Number(getComputedStyle(b).order) || 0));
  if (side === "left") cells.reverse();
  const first = cells[0] as HTMLElement | undefined;
  const cellStyle = first ? getComputedStyle(first) : null;
  const left = parseFloat(cellStyle?.paddingLeft ?? "") || 12;
  const right = parseFloat(cellStyle?.paddingRight ?? "") || 12;
  const size = first ? first.offsetWidth - left - right : 56;
  const margin = side === "left" ? right : left;
  const gap = margin;
  const width = margin * 2 + cells.length * size + Math.max(0, cells.length - 1) * gap;
  return {
    cells,
    side,
    gap,
    fullWidth: width + CHEVRON_WIDTH,
    margin,
    openAt: margin + size + gap + CHEVRON_WIDTH,
    revealAt: Math.max(1, Math.min(size - 1, intAttr(row, "data-swipe-reveal-at-px", REVEAL_AT_PX))),
    size,
    width,
  };
}

function setMotionValue(element: HTMLElement, name: string, value: number | string): void {
  const text = String(value);
  if (element.style.getPropertyValue(name) !== text) element.style.setProperty(name, text);
}

export function paintSwipeMotion(tray: HTMLElement, motion: SwipeMotion, distance: number): void {
  const { cells, gap, margin, revealAt, size } = motion;
  const dragging = tray.closest("[data-swipe]")?.hasAttribute("data-swipe-dragging") === true;
  const reserve = dragging ? motion.fullWidth - motion.width : 0;
  const chevronVisible = dragging && distance >= motion.openAt;
  setMotionValue(tray, "--swipe-cue-padding", (margin / 2) + "px");
  setMotionValue(tray, "--swipe-chevron-reserve", chevronVisible ? reserve : 0);
  const extraWidth = Math.max(0, distance - motion.width - reserve);
  for (const [index, cell] of cells.entries()) {
    const outer = index === 0 ? margin : gap / 2;
    const inner = index === cells.length - 1 ? margin : gap / 2;
    setMotionValue(cell, "--swipe-pad-left", (motion.side === "left" ? inner : outer) + "px");
    setMotionValue(cell, "--swipe-pad-right", (motion.side === "left" ? outer : inner) + "px");
    setMotionValue(cell, "--swipe-slot-width", (size + outer + inner + extraWidth / Math.max(1, cells.length)) + "px");
    const available = Math.max(0, Math.min(size, distance - margin * 2 - index * (size + gap) - (index ? reserve : 0)));
    const scale = available >= revealAt ? available / size : 0;
    const opacity = Math.max(0, (available - revealAt) / (size - revealAt));
    setMotionValue(cell, "--swipe-circle-shift", ((motion.side === "left" ? 1 : -1) * (size - available) / 2) + "px");
    setMotionValue(cell, "--swipe-scale", scale);
    setMotionValue(cell, "--swipe-opacity", opacity);
  }
  setMotionValue(tray, "--swipe-chevron-opacity", chevronVisible ? 1 : 0);
}
