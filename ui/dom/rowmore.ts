import { activateMenu } from "../components/menu/menu";
import { layer } from "./keys";
import { registerComponent } from "./lifecycle";
import { HOLD_EXEMPT } from "./pan";
import { touch } from "./viewport";

// A row's swipe-tray More button or long press opens a modal card listing
// all of the row's actions, one per line with its glyph
// and label. Choosing one closes the card and clicks the row's own button, so
// whatever handles a tap on that button -- an optimistic update, a toast, an
// Undo, a menu anchored to it -- handles the choice the same way. The card is
// built from the buttons each time it opens, so a stateful button (Mark read /
// Mark unread) is listed under its current label.

const TRIGGER = "[data-swipe-more]";
const TRAY = "[data-swipe-part=tray]";
const ROW = "[data-slot=mail-row]";
const CLUSTER = "[data-slot=hover-actions]";
const ACTION = "[data-slot=button-icon], [data-slot=menu-trigger]";

// Classes are written out whole so Tailwind finds them here. A long press
// opens the card under a finger still down, where the browser's own long
// press would select its summary and keep the taps that follow, so nothing in
// it selects.
const DIALOG_CLASS =
  "scrim-dialog select-none [-webkit-touch-callout:none] m-auto w-[calc(100%-2rem)] max-h-[calc(100dvh-2rem)] max-w-none overflow-y-auto rounded-2xl border border-border bg-popover p-0 text-popover-foreground shadow-xl";
export const CARD_LIST_CLASS = "flex flex-col py-1.5";
export const CARD_ITEM_CLASS =
  "flex min-h-12 w-full cursor-pointer items-center gap-3 px-4 text-left type-body-300 text-foreground active:bg-muted focus-visible:bg-muted focus-visible:outline-none [&_svg]:size-5 [&_svg]:flex-none [&_svg]:text-muted-foreground";
const CANCEL_CLASS =
  "flex min-h-12 w-full cursor-pointer items-center justify-center border-t border-border px-4 type-body-300-medium text-muted-foreground active:bg-muted focus-visible:bg-muted focus-visible:outline-none";

// The card is a layer of the key router (ui/dom/keys.ts), so a card opened
// over a modal layer -- the phone's navigation sheet, whose router holds the
// rest of the page inert -- stacks over it and stays live. Otherwise the
// router counts the card, a child of the body, as page under the sheet and
// makes it inert, and no tap or click reaches its lines. Escape closes the
// card, not the layer under it.
layer({
  escape: (_event, el) => {
    if (el instanceof HTMLDialogElement) el.close();
    return true;
  },
  id: "row-more",
  match: "[data-slot=row-more-dialog][open]",
  regions: (el) => [el ?? null],
});

let dialog: HTMLDialogElement | null = null;
// The element the card is open for -- a mail row, or the opener of a menu
// shown as the card (ui/dom/menucard.ts) -- and what to do when it closes.
let activeRow: HTMLElement | null = null;
let onClosed: (() => void) | null = null;

function ensureDialog(): HTMLDialogElement {
  if (dialog?.isConnected === true) return dialog;
  dialog = document.createElement("dialog");
  dialog.setAttribute("data-slot", "row-more-dialog");
  dialog.setAttribute("aria-label", "Actions");
  dialog.className = DIALOG_CLASS;
  dialog.addEventListener("close", () => {
    if (dialog?.open === true) return;
    activeRow = null;
    const closed = onClosed;
    onClosed = null;
    closed?.();
  });
  dialog.addEventListener("click", (e) => {
    if (e.target === dialog) dialog?.close();
  });
  document.body.appendChild(dialog);
  return dialog;
}

// CardIn is what the card shows: parts above a Cancel line, for owner, under
// label as its accessible name; closed runs once it shuts, however it does.
export type CardIn = { closed?: () => void; label: string; owner: HTMLElement; parts: Node[] };

// cardOpenFor says whether the card is open for owner.
export function cardOpenFor(owner: HTMLElement): boolean {
  return dialog?.open === true && activeRow === owner;
}

export function closeCard(): void {
  dialog?.close();
}

// openCard shows the card with card's parts, closing any card already open
// first. That one's close event -- or that of a card closed earlier in this
// task, which has not come yet either -- comes after this has opened the
// dialog again, and is ignored then, so its closed runs here. The card hands
// focus back, when it closes, to whatever held it as it opened.
export function openCard(card: CardIn): void {
  const d = ensureDialog();
  const closed = onClosed;
  onClosed = null;
  activeRow = null;
  if (d.open) d.close();
  closed?.();
  d.replaceChildren();
  d.setAttribute("aria-label", card.label);
  const cancel = document.createElement("button");
  cancel.type = "button";
  cancel.className = CANCEL_CLASS;
  cancel.textContent = "Cancel";
  cancel.addEventListener("click", () => d.close());
  d.append(...card.parts, cancel);
  activeRow = card.owner;
  onClosed = card.closed ?? null;
  d.showModal();
}

function labelOf(button: HTMLElement): string {
  if (button.hasAttribute("data-swipe-star") && button.getAttribute("aria-pressed") === "true") return "Unstar";
  return (button.getAttribute("aria-label") || button.textContent || "").trim();
}

// glyphOf copies the button's visible glyph: a stateful button renders every
// state's and hides all but the current one.
function glyphOf(button: HTMLElement): Node | null {
  for (const svg of Array.from(button.querySelectorAll("svg"))) {
    if (svg.closest("[hidden]")) continue;
    return svg.cloneNode(true);
  }
  return null;
}

function summaryOf(row: HTMLElement): HTMLElement | null {
  const source = row.querySelector<HTMLElement>("[data-slot=mail-row-summary]");
  if (!source) return null;
  const summary = source.cloneNode(true);
  if (!(summary instanceof HTMLElement)) return null;
  summary.querySelectorAll("[data-slot=mail-row-more], [data-slot=avatar], [popover], template").forEach(node =>
    node.remove()
  );
  const star = row.querySelector("[data-slot=star]");
  if (star?.getAttribute("aria-pressed") === "true") {
    summary.querySelector("[data-slot=mail-row-star-indicator]")?.classList.add("max-docked:inline-flex");
  }
  // The current rendered summary supplies styling and optimistic state. Its
  // static copy must not mount controls or retain references to live-row IDs.
  for (const node of [summary, ...summary.querySelectorAll("*")]) {
    for (const attribute of Array.from(node.attributes)) {
      const name = attribute.name;
      if (
        name === "id" || name === "href" || name === "tabindex" || name === "type"
        || name === "popovertarget" || name === "aria-controls" || name === "aria-labelledby"
        || name === "aria-describedby" || name === "aria-haspopup" || name === "aria-expanded"
        || name.startsWith("on") || name.startsWith("hx-") || name.startsWith("data-")
      ) node.removeAttribute(name);
    }
    if (node.matches("a, button")) {
      const text = document.createElement("span");
      for (const attribute of Array.from(node.attributes)) text.setAttribute(attribute.name, attribute.value);
      text.append(...node.childNodes);
      node.replaceWith(text);
    }
  }
  summary.setAttribute("data-slot", "mail-row-summary");
  const header = document.createElement("div");
  header.className = "group/mail-row border-b border-border py-1";
  header.setAttribute("data-slot", "row-more-summary");
  header.setAttribute("data-read-state", row.getAttribute("data-read-state") ?? "read");
  header.appendChild(summary);
  return header;
}

export function openRowActions(row: HTMLElement): void {
  if (cardOpenFor(row)) return;
  const clusters = row.querySelectorAll(CLUSTER);
  const actions = Array.from(
    new Set(Array.from(clusters).flatMap(cluster => Array.from(cluster.querySelectorAll<HTMLElement>(ACTION)))),
  )
    .filter(
      (b) =>
        b.closest(ROW) === row && !b.closest("[role=menu], [popover]")
        && !b.matches(TRIGGER) && !(b instanceof HTMLButtonElement && b.disabled)
        && b.getAttribute("aria-disabled") !== "true",
    );
  if (!actions.length) return;
  const list = document.createElement("div");
  list.className = CARD_LIST_CLASS;
  for (const action of actions) {
    const item = document.createElement("button");
    item.type = "button";
    item.className = CARD_ITEM_CLASS;
    item.setAttribute("data-slot", "row-more-item");
    const glyph = glyphOf(action);
    if (glyph) item.appendChild(glyph);
    const label = document.createElement("span");
    label.textContent = labelOf(action);
    item.appendChild(label);
    item.addEventListener("click", () => {
      closeCard();
      // After the close, so focus has returned to the trigger and a menu the
      // action opens keeps the focus it takes.
      setTimeout(() => {
        if (action.isConnected) {
          if (action.matches("[data-slot=menu-trigger]")) {
            activateMenu(action, row);
          } else action.click();
        }
      }, 0);
    });
    list.appendChild(item);
  }
  const summary = summaryOf(row);
  // The card hands focus back, when it closes, to whatever held it as it
  // opened. A swipe tray's More, or a long press with focus elsewhere, would
  // get it back on a tray action, which reopens the row (ui/dom/swipe.ts)
  // with its tooltip showing; the row itself takes it instead, so the tray
  // stays shut. A More in sight keeps it.
  const focused = document.activeElement;
  if (!(focused instanceof HTMLElement) || !row.contains(focused) || focused.closest(TRAY)) {
    row.focus({ preventScroll: true, focusVisible: false });
  }
  openCard({ label: "Actions", owner: row, parts: summary ? [summary, list] : [list] });
}

// A long press on a touch screen is the row's, as ui/dom/pan.ts's hold is,
// links included: the row's sender, subject and preview are its links. Its
// own controls, fields and popovers keep the browser's (pan.HOLD_EXEMPT).
registerComponent(ROW, (row, signal) => {
  row.addEventListener("contextmenu", (e) => {
    if (!touch().matches) return;
    if (e.target instanceof Element && e.target.closest(HOLD_EXEMPT)) return;
    e.preventDefault();
    openRowActions(row);
  }, { signal });
  signal.addEventListener("abort", () => {
    if (activeRow === row) dialog?.close();
  }, { once: true });
});

document.addEventListener("click", (e) => {
  const trigger = e.target instanceof Element ? e.target.closest<HTMLElement>(TRIGGER) : null;
  if (!trigger) return;
  e.preventDefault();
  const row = trigger.closest<HTMLElement>(ROW);
  if (row) openRowActions(row);
}, true);
