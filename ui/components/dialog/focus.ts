import { settle } from "../../dom/keys";
import { registerComponent } from "../../dom/lifecycle";
import { openerOf } from "../menu/menu";

const DIALOG = 'dialog[data-slot="dialog"]';
type Opening = { dialog: HTMLDialogElement; fallback: HTMLElement | null; opener: HTMLElement | null };
const openings = new Map<string, Opening>();
let swapOpener: HTMLElement | null = null;

function fallbackFor(opener: HTMLElement | null): HTMLElement | null {
  const menu = opener?.closest('[data-slot="menu"]');
  const trigger = menu?.querySelector<HTMLElement>('[data-slot="menu-trigger"]');
  return trigger ? openerOf(trigger) : null;
}

export function rememberOpening(dialog: HTMLDialogElement, invoker?: HTMLElement): void {
  const previous = openings.get(dialog.id);
  const kept = !invoker && previous && previous.dialog !== dialog && !previous.dialog.isConnected
    ? previous
    : undefined;
  if (!kept) openings.delete(dialog.id);
  const active = invoker ?? swapOpener ?? document.activeElement;
  const opener = kept?.opener
    ?? (active instanceof HTMLElement && active !== document.body && !dialog.contains(active) ? active : null);
  openings.set(dialog.id, { dialog, fallback: kept?.fallback ?? fallbackFor(opener), opener });
}

// Native showModal appends to the top layer. A replacement retains its place
// below dialogs opened after it, including a dialog elsewhere in the DOM.
export function restoreOrder(dialog: HTMLDialogElement): void {
  const entries = Array.from(openings.values());
  const index = entries.findIndex(entry => entry.dialog === dialog);
  const above = entries.slice(index + 1).map(entry => entry.dialog).filter(other => other.isConnected && other.open);
  if (!above.length) return;
  for (const other of above) other.close();
  settle();
  for (const other of above) other.showModal();
  settle();
}

function returnTo(entry: Opening): void {
  if (entry.opener?.isConnected === true) entry.opener.focus({ preventScroll: true });
  if (document.activeElement !== entry.opener && entry.fallback?.isConnected === true) {
    entry.fallback.focus({ preventScroll: true });
  }
}

export function closeDialog(dialog: HTMLDialogElement): void {
  if (!dialog.open) return;
  const entry = openings.get(dialog.id);
  const returnFocus = dialog.contains(document.activeElement);
  dialog.close();
  settle();
  if (entry?.dialog === dialog) {
    openings.delete(dialog.id);
    if (returnFocus) returnTo(entry);
  }
}

registerComponent(DIALOG, (dialog, signal) => {
  if (!(dialog instanceof HTMLDialogElement)) return;
  dialog.addEventListener("close", () => {
    if (dialog.open) return;
    const entry = openings.get(dialog.id);
    if (entry?.dialog !== dialog) return;
    openings.delete(dialog.id);
    const active = document.activeElement;
    if (active !== document.body && active !== entry.opener && active !== entry.fallback && !dialog.contains(active)) {
      return;
    }
    settle();
    returnTo(entry);
  }, { signal });
  signal.addEventListener("abort", () => {
    if (openings.get(dialog.id)?.dialog === dialog) openings.delete(dialog.id);
  }, { once: true });
});

document.addEventListener("htmx:before:swap", (event) => {
  const active = document.activeElement;
  const source = event.detail?.ctx?.sourceElement;
  swapOpener = active instanceof HTMLElement && active !== document.body
    ? active
    : source instanceof HTMLElement
    ? source
    : null;
});
document.addEventListener("htmx:finally:swap", () => {
  swapOpener = null;
});
