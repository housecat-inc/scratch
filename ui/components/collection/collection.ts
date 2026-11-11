import { eventOwner } from "../../dom/housecat";
import { activates, arrows, editable, isMac } from "../../dom/keyinfo";
import { on } from "../../dom/keys";
import { registerComponent } from "../../dom/lifecycle";

const ROOT = '[data-collection="root"]';
const ROW = '[data-collection="row"]';
const CELL = '[data-collection="cell"]';
const TRIGGER = '[data-collection="trigger"]';
const ACTION = '[data-collection="action"]';
const CONTROL = 'a[href],button,input,select,textarea,summary,[tabindex],[contenteditable="true"]';
const POPUP = '[popover],dialog,[role="dialog"],[role="alertdialog"],[role="menu"]';
const originalTabIndex = new WeakMap<HTMLElement, string | null>();
const positions = new WeakMap<Element, HTMLElement>();
const editing = new WeakMap<Element, HTMLElement>();
const rowFocus = new WeakMap<Element, HTMLElement>();

function rowNavigation(root: Element): boolean {
  return root.getAttribute("data-navigation") === "row";
}

function visible(el: HTMLElement): boolean {
  if (el.closest('[hidden],[inert],[aria-disabled="true"]') || el.hasAttribute("disabled")) return false;
  return el.getClientRects().length > 0 || getComputedStyle(el).display === "contents"
      && Array.from(el.children).some((child) => child.getClientRects().length > 0);
}
// header is a column header: announced as one, but never a stop in the list,
// so the controls in it, such as a table's own actions, keep their place in
// the page's Tab order.
function header(cell: Element): boolean {
  return cell.getAttribute("role") === "columnheader";
}
function cells(row: Element): HTMLElement[] {
  return Array.from(row.querySelectorAll<HTMLElement>(CELL)).filter((cell) =>
    cell.closest(ROW) === row && visible(cell) && !header(cell)
  );
}
function isItem(root: Element, row: HTMLElement): boolean {
  return row.closest(ROOT) === root && visible(row) && cells(row).length > 0;
}
// rowOf is the grid row that holds row: the wrapper the list's collection
// puts around each entry, such as Mail's, or row itself when it is the grid
// row. A script that takes an entry out takes its grid row with it, so the
// grid keeps no empty row.
export function rowOf(row: HTMLElement): HTMLElement {
  return row.closest<HTMLElement>(ROW) ?? row;
}
export function items(root: Element): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(ROW)).filter((row) => isItem(root, row));
}
function popupWithin(el: Element, surface: Element): boolean {
  const popup = el.closest(POPUP);
  return !!popup && surface.contains(popup);
}
function controls(cell: HTMLElement): HTMLElement[] {
  return Array.from(cell.querySelectorAll<HTMLElement>(CONTROL)).filter((el) =>
    el.closest(CELL) === cell && el.closest(ROOT) === cell.closest(ROOT) && !popupWithin(el, cell) && visible(el)
    && getComputedStyle(el).display !== "contents"
  );
}
function trigger(cell: HTMLElement): HTMLElement | undefined {
  return controls(cell).find((el) => el.matches(TRIGGER));
}
function destination(cell: HTMLElement): HTMLElement {
  const available = controls(cell);
  return trigger(cell) ?? (available.length === 1 && !editable(available[0]) ? available[0] : cell);
}
function primary(row: HTMLElement): HTMLElement {
  const available = cells(row);
  return destination(available.find(cell => !!trigger(cell)) ?? available[0]);
}
function rowControls(row: HTMLElement): HTMLElement[] {
  const available = cells(row).flatMap(controls);
  const first = primary(row);
  return available.includes(first) ? available : [first, ...available];
}
// focusTarget is where focus goes in cell: in row mode the control the row
// holds, which Left and Right move among, falling back to the cell's
// destination; in cell mode always that destination.
function focusTarget(root: Element, cell: HTMLElement): HTMLElement {
  return (rowNavigation(root) ? rowFocus.get(root) : undefined) ?? destination(cell);
}
function writeTabIndex(el: HTMLElement, value: number): void {
  if (!originalTabIndex.has(el)) originalTabIndex.set(el, el.getAttribute("tabindex"));
  if (el.tabIndex !== value) el.tabIndex = value;
}
function arrange(root: Element, cell: HTMLElement): void {
  const active = editing.get(root);
  root.querySelectorAll<HTMLElement>(CELL).forEach((each) => {
    if (each.closest(ROOT) !== root || header(each)) return;
    writeTabIndex(each, -1);
    each.querySelectorAll<HTMLElement>(CONTROL).forEach((el) => {
      if (el.closest(CELL) !== each || el.closest(ROOT) !== root || popupWithin(el, each)) return;
      writeTabIndex(el, active === each && originalTabIndex.get(el) !== "-1" ? 0 : -1);
    });
  });
  if (!active) writeTabIndex(focusTarget(root, cell), 0);
}
// current measures only the rows marked current, not every row in the list:
// it runs on each change, hover and key the list sees.
export function current(root: Element | null): HTMLElement | null {
  if (!root) return null;
  return Array.from(root.querySelectorAll<HTMLElement>(`${ROW}[data-current]`)).find((row) => isItem(root, row))
    ?? null;
}
type SelectOptions = { focus?: boolean; scroll?: boolean; target?: HTMLElement };
function focusCell(cell: HTMLElement, options: SelectOptions = {}): HTMLElement | null {
  const root = cell.closest(ROOT);
  const row = cell.closest<HTMLElement>(ROW);
  if (!root || !row || !visible(cell)) return null;
  const previous = current(root);
  if (rowNavigation(root)) {
    const held = rowFocus.get(root);
    rowFocus.set(
      root,
      options.target ?? (options.focus !== true && held && cell.contains(held) && visible(held)
        ? held
        : destination(cell)),
    );
  }
  positions.set(root, cell);
  root.querySelectorAll(ROW).forEach((each) => {
    if (each.closest(ROOT) === root) {
      if (each.hasAttribute("data-current") !== (each === row)) each.toggleAttribute("data-current", each === row);
    }
  });
  arrange(root, cell);
  if (options.focus === true) focusTarget(root, cell).focus({ preventScroll: true });
  if (options.scroll !== false) {
    (cell.getClientRects().length ? cell : row).scrollIntoView({ block: "nearest", inline: "nearest" });
  }
  if (previous !== row) {
    root.dispatchEvent(
      new CustomEvent("collection:current-change", { bubbles: true, detail: { item: row, previous } }),
    );
  }
  return row;
}
export function select(row: HTMLElement | null, options: SelectOptions = {}): HTMLElement | null {
  row = row?.closest<HTMLElement>(ROW) ?? null;
  if (!row) return null;
  const available = cells(row);
  const first = available[0] as HTMLElement | undefined;
  const cell = available.find((each) => !!trigger(each)) ?? first;
  return cell ? focusCell(cell, options) : null;
}
export function move(root: Element | null, by: number): HTMLElement | null {
  if (!root) return null;
  const rows = items(root);
  const selected = current(root);
  const at = selected ? rows.indexOf(selected) : by > 0 ? -1 : rows.length;
  const next = rows[Math.max(0, Math.min(rows.length - 1, at + by))] as HTMLElement | undefined;
  if (!next) return null;
  if (rowNavigation(root)) return select(next, { focus: true });
  const column = selected ? cells(selected).indexOf(positions.get(root)!) : 0;
  const available = cells(next);
  return focusCell(available[Math.max(0, Math.min(column, available.length - 1))], { focus: true });
}
export function navigating(target: HTMLElement): boolean {
  const cell = target.closest<HTMLElement>(CELL);
  const root = cell?.closest(ROOT);
  return !!cell && !!root && !editing.has(root) && !popupWithin(target, root)
    && (rowNavigation(root) ? !arrows(target, cell) : target === cell || target === destination(cell));
}

// level is a row's depth in a tree of rows, such as the Mail settings
// sidebar, whose section rows carry aria-level 1 and entries 2.
function level(row: Element): number {
  return Number(row.getAttribute("aria-level")) || 1;
}
// expand opens or closes a tree's section row through the control in it that
// toggles it, and is false when the row is not a section or already is so.
function expand(row: HTMLElement, open: boolean): boolean {
  if (row.getAttribute("aria-expanded") !== String(!open)) return false;
  const toggle = Array.from(row.querySelectorAll<HTMLElement>("[aria-expanded]")).find((el) => el.closest(ROW) === row);
  toggle?.click();
  return !!toggle;
}

function navigateRow(event: KeyboardEvent, root: Element, cell: HTMLElement, target: Element): boolean {
  const row = cell.closest<HTMLElement>(ROW)!;
  const first = primary(row);
  if (event.key === "Escape") return !!select(row, { focus: true });
  if (event.altKey || event.shiftKey || arrows(target, cell)) return false;
  // Rows move from the focused row. A script that made another row current
  // without moving focus, such as a failed action putting its row back, is
  // caught up first.
  const step = (by: number): boolean => {
    if (!row.hasAttribute("data-current")) {
      focusCell(cell, target instanceof HTMLElement ? { scroll: false, target } : { scroll: false });
    }
    return !!move(root, by);
  };
  // Command with Up or Down on a Mac, as Home and End, reach the first and
  // last rows; any other Command key stays with the control.
  if (event.metaKey) {
    if (!isMac() || event.ctrlKey || event.key !== "ArrowUp" && event.key !== "ArrowDown") return false;
    return step(event.key === "ArrowUp" ? -Infinity : Infinity);
  }
  if (event.ctrlKey && event.key !== "Home" && event.key !== "End") return false;
  let next: HTMLElement | undefined;
  const available = rowControls(row);
  switch (event.key) {
    case "ArrowUp":
      return step(-1);
    case "ArrowDown":
      return step(1);
    case "PageUp":
      return step(-10);
    case "PageDown":
      return step(10);
    case "Home":
      return !!select(items(root).at(0) ?? null, { focus: true });
    case "End":
      return !!select(items(root).at(-1) ?? null, { focus: true });
    case "ArrowLeft":
    case "ArrowRight": {
      const at = (target instanceof HTMLElement ? available.indexOf(target) : -1)
        + (event.key === "ArrowLeft" ? -1 : 1);
      next = at >= 0 ? available.at(at) : undefined;
      if (next) break;
      // In a tree of rows, Left at a row's primary target closes an open
      // section row, or climbs to the row's section row; Right on a closed
      // section row opens it, and past an open one's last control goes on
      // to its first entry.
      if (event.key === "ArrowLeft") {
        if (expand(row, false)) return true;
        const rows = items(root);
        const parent = rows.slice(0, rows.indexOf(row)).reverse().find((each) => level(each) < level(row));
        return !!parent && !!select(parent, { focus: true });
      }
      if (target === first && expand(row, true)) return true;
      return row.getAttribute("aria-expanded") === "true" && step(1);
    }
    case "F2":
      next = target === first ? available.find(control => control !== first) : first;
      break;
    case "Enter": {
      if (target !== first || activates(target)) return false;
      const action = row.querySelector<HTMLElement>(ACTION);
      if (!action || !visible(action)) return false;
      action.click();
      return true;
    }
    default:
      return false;
  }
  if (next) focusCell(next.closest<HTMLElement>(CELL)!, { focus: true, target: next });
  return true;
}

// VERTICAL are the keys that move between rows from anywhere in a row but a
// control with arrow keys of its own.
const VERTICAL = ["ArrowDown", "ArrowUp", "PageDown", "PageUp"];

function enter(root: Element, cell: HTMLElement, next?: HTMLElement): boolean {
  const available = controls(cell);
  if (!available.length) return false;
  editing.set(root, cell);
  arrange(root, cell);
  (next ?? available.find((el) => el !== document.activeElement) ?? available[0]).focus();
  return true;
}
function leave(root: Element, cell: HTMLElement): boolean {
  editing.delete(root);
  focusCell(cell, { focus: true });
  return true;
}

function stepControl(root: Element, cell: HTMLElement, target: Element, by: number): boolean {
  const available = controls(cell);
  const at = target instanceof HTMLElement ? available.indexOf(target) : -1;
  const next = available[at + by] as HTMLElement | undefined;
  if (next) {
    if (editing.has(root) && next === destination(cell)) return leave(root, cell);
    return enter(root, cell, next);
  }
  return editing.has(root) && by < 0 ? leave(root, cell) : false;
}

// shown is whether the list draws row as its current row where it is now: a
// list that does renders something in the row, the row indicator, that sets
// --collection-current: shown. Mail draws it from md up.
function shown(row: HTMLElement): boolean {
  return Array.from(row.querySelectorAll<HTMLElement>("*")).some((el) =>
    el.closest(ROW) === row && el.getClientRects().length > 0
    && getComputedStyle(el).getPropertyValue("--collection-current").trim() === "shown"
  );
}
// enterList moves focus onto a list's rows from outside them. A current row
// the list already shows is where the keys pick up, so the first press moves
// on from it, as it would with focus on it. One not shown is where focus
// lands, and with none, the first row, or going up the last.
function enterList(root: Element, up: boolean): boolean {
  const rows = items(root);
  const at = current(root);
  if (at && shown(at)) return !!move(root, up ? -1 : 1);
  const row = at ?? (up ? rows.at(-1) : rows.at(0));
  return !!select(row ?? null, { focus: true });
}

on(ROOT, (event, root, target) => {
  if (!(target instanceof Element) || target.closest(ROOT) !== root || popupWithin(target, root)) return false;
  const cell = target.closest<HTMLElement>(CELL);
  if (!cell) return false;
  // A column header, or a control in one, is not a navigable cell: arrows
  // from it would make the header row current and its tab stop. Up and Down
  // from it go on into the rows, as they do from off the list.
  if (header(cell)) {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return false;
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return false;
    return enterList(root, event.key === "ArrowUp");
  }
  if (rowNavigation(root)) return navigateRow(event, root, cell, target);
  const owned = arrows(target, cell);
  if (editing.has(root)) {
    if (event.key === "Escape" || event.key === "F2") return leave(root, cell);
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || owned) return false;
    if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
      stepControl(root, cell, target, event.key === "ArrowLeft" ? -1 : 1);
      return true;
    }
    // Only a key that moves rows leaves the entered cell.
    if (!VERTICAL.includes(event.key)) return false;
    editing.delete(root);
  }
  if (event.key === "F2") return enter(root, cell);
  // Escape from any other control in a cell, such as one a click focused,
  // returns to the cell as it does from F2's.
  if (event.key === "Escape" && target !== cell && target !== destination(cell)) return leave(root, cell);
  if (owned) return false;
  if (event.altKey || event.metaKey || event.shiftKey) return false;
  // Up, Down, Page Up and Page Down move rows from any control in the cell;
  // an own-events control keeps every other key.
  if (
    !VERTICAL.includes(event.key) && target !== destination(cell) && target !== cell && eventOwner(event, cell)
  ) return false;
  const row = cell.closest<HTMLElement>(ROW)!;
  const rowCells = cells(row);
  const at = rowCells.indexOf(cell);
  let next: HTMLElement | undefined;
  switch (event.key) {
    case "ArrowDown":
      return !event.ctrlKey && !!move(root, 1);
    case "ArrowUp":
      return !event.ctrlKey && !!move(root, -1);
    case "PageDown":
      return !event.ctrlKey && !!move(root, 10);
    case "PageUp":
      return !event.ctrlKey && !!move(root, -10);
    case "ArrowLeft":
      if (!event.ctrlKey && target !== cell && stepControl(root, cell, target, -1)) return true;
      next = rowCells.slice(0, at).reverse().find((each) => controls(each).length > 0) ?? rowCells[0];
      break;
    case "ArrowRight":
      if (!event.ctrlKey && stepControl(root, cell, target, 1)) return true;
      next = rowCells.slice(at + 1).find((each) => controls(each).length > 0) ?? cell;
      break;
    // A hidden cell is not among its row's cells, which may then be none.
    case "Home":
      next = event.ctrlKey ? cells(items(root)[0])[0] : rowCells[0] as HTMLElement | undefined;
      break;
    case "End":
      if (event.ctrlKey) {
        // A collection with no rows has no last cell.
        const lastRow = items(root).at(-1);
        next = lastRow ? cells(lastRow).at(-1) : undefined;
      } else {
        next = rowCells.at(-1);
      }
      break;
    case "Enter":
      return !activates(target) && enter(root, cell);
    default:
      return false;
  }
  return !!next && !!focusCell(next, { focus: true });
}, { typing: "always" });

// enterFromPage starts arrow navigation on the page's first list of entries
// that opted in (data-enter-from-page), such as Mail's, when focus is
// anywhere off a list: on the page itself, a sidebar link, a plain button.
function enterFromPage(event: KeyboardEvent, _body: Element, target: Element): boolean {
  if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return false;
  // A table's header sits outside the list its body is, so Up and Down from
  // a column header, or a control in one, enter that table's rows.
  const head = target.closest("thead, [role=columnheader]");
  if (head) {
    const root = head.closest("table")?.querySelector<HTMLElement>(ROOT);
    return !!root && visible(root) && enterList(root, event.key === "ArrowUp");
  }
  // A list, a popup and a control with arrow keys of its own keep them. On a
  // page with no list that opted in, such as a settings table, the arrows
  // scroll the page and Tab enters the list.
  if (target.closest(ROOT) || target.closest(POPUP) || arrows(target)) return false;
  for (const root of Array.from(document.querySelectorAll<HTMLElement>(ROOT + "[data-enter-from-page]"))) {
    if (visible(root) && items(root).length) return enterList(root, event.key === "ArrowUp");
  }
  return false;
}
on("body", { ArrowDown: enterFromPage, ArrowUp: enterFromPage }, {
  id: "collection-enter",
  label: "Move through the list",
  typing: "never",
});

// captures is whether focus is on something that keeps the keyboard, which
// hovering a list must leave alone: a field, a frame, a popup, or a widget
// with arrow keys of its own other than a list. A field outside the list --
// All Tools, a composer -- also acts on the current row, so passing the
// pointer over the list on the way changes nothing at all.
function captures(active: Element | null): boolean {
  if (!active || active === document.body || active === document.documentElement) return false;
  return editable(active) || active.tagName === "IFRAME" || !!active.closest(POPUP)
    || arrows(active) && !active.closest(ROOT);
}

// Hovering a row makes it current and moves focus to it, so the keys act on
// the row under the pointer. That includes the row's own buttons, such as a
// star the pointer can reach without crossing the rest of the row. A control
// reached from the keyboard keeps its row; one a click focused hands the row
// back to the pointer, whose hand is on the mouse.
document.addEventListener("pointermove", (event) => {
  if (event.pointerType !== "mouse" || event.buttons || !(event.target instanceof Element)) return;
  const row = event.target.closest<HTMLElement>(ROW);
  const root = row?.closest<HTMLElement>(ROOT);
  if (!row || root?.hasAttribute("data-hover-current") !== true) return;
  const active = document.activeElement;
  if (row.hasAttribute("data-current") && !!active && row.contains(active)) return;
  // Passing over a header row on the way to the rows changes nothing.
  if (!cells(row).length || popupWithin(event.target, root)) return;
  if (active && popupWithin(active, root) || captures(active)) return;
  if (rowNavigation(root)) {
    const activeRow = active?.closest<HTMLElement>(ROW);
    if (
      activeRow && root.contains(activeRow) && active !== primary(activeRow)
      && active?.matches(":focus-visible") === true
    ) {
      return;
    }
  } else if (editing.has(root)) {
    if (active?.matches(":focus-visible") === true) return;
    editing.delete(root);
  }
  select(row, { focus: true, scroll: false });
});

document.addEventListener("focusin", (event) => {
  if (!(event.target instanceof HTMLElement)) return;
  const cell = event.target.closest<HTMLElement>(CELL);
  const root = cell?.closest(ROOT);
  if (!cell || !root || popupWithin(event.target, root) || header(cell)) return;
  // In cell mode, focus a click puts on a control with arrow keys of its own
  // enters the cell, as F2 would; any other control leaves the grid
  // navigating.
  if (!rowNavigation(root)) {
    if (editing.get(root) !== cell) editing.delete(root);
    if (event.target !== destination(cell) && event.target !== cell && arrows(event.target, cell)) {
      editing.set(root, cell);
    }
  }
  focusCell(cell, { scroll: false, target: event.target });
});

registerComponent(ROOT, (root, signal) => {
  let frame = 0;
  let previousIndex = 0;
  let hadFocus = false;
  root.addEventListener("focusin", () => {
    hadFocus = true;
  }, { signal });
  root.addEventListener("focusout", (event) => {
    const from = event.target;
    const next = event.relatedTarget;
    if (next instanceof Node && root.contains(next)) return;
    const release = () => {
      if (root.contains(document.activeElement)) return;
      if (from instanceof HTMLElement && (!from.isConnected || !visible(from))) return;
      hadFocus = false;
      editing.delete(root);
      const cell = positions.get(root);
      if (cell) arrange(root, cell);
    };
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(() => {
      if (!signal.aborted) release();
    });
  }, { signal });
  const reconcile = () => {
    const rows = items(root);
    const previous = rows[Math.min(previousIndex, rows.length - 1)] as HTMLElement | undefined;
    const next = current(root) ?? previous ?? null;
    const cell = positions.get(root);
    if (!next) {
      positions.delete(root);
      editing.delete(root);
      if (root instanceof HTMLElement) {
        if (root.tabIndex !== 0) root.tabIndex = 0;
        if (cell && hadFocus) root.focus();
      }
      return;
    }
    if (root.hasAttribute("tabindex")) root.removeAttribute("tabindex");
    const held = rowFocus.get(root);
    if (rowNavigation(root) && held && hadFocus && (!held.isConnected || !visible(held))) {
      select(next, { focus: true, scroll: false });
    } else if (cell && cell.isConnected && visible(cell)) focusCell(cell, { scroll: false });
    else {
      editing.delete(root);
      select(next, { focus: !!cell && hadFocus, scroll: false });
    }
    previousIndex = rows.indexOf(next);
  };
  root.addEventListener("collection:current-change", () => {
    previousIndex = items(root).indexOf(current(root)!);
  }, { signal });
  const observer = new MutationObserver(reconcile);
  observer.observe(root, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["hidden", "inert", "aria-disabled"],
  });
  const resize = new ResizeObserver(reconcile);
  resize.observe(root);
  signal.addEventListener("abort", () => {
    observer.disconnect();
    resize.disconnect();
    cancelAnimationFrame(frame);
    positions.delete(root);
    editing.delete(root);
    rowFocus.delete(root);
  }, { once: true });
  reconcile();
});

window.HousecatCollection = { current, items, move, rowOf, select };
declare global {
  interface Window {
    HousecatCollection: {
      current: typeof current;
      items: typeof items;
      move: typeof move;
      rowOf: typeof rowOf;
      select: typeof select;
    };
  }
}
