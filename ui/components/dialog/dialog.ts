import { layer, settle } from "../../dom/keys";
import { registerComponent } from "../../dom/lifecycle";

import { closeDialog, rememberOpening, restoreOrder } from "./focus";
import "./scroll";

const DIALOG = 'dialog[data-slot="dialog"]';

layer({
  escape: (_event, element) => {
    if (element instanceof HTMLDialogElement) closeDialog(element);
    return true;
  },
  id: "dialog",
  identity: "id",
  match: DIALOG + "[open]",
  regions: (element) => [element ?? null],
});

let backdropPress: HTMLDialogElement | null = null;

function outside(dialog: HTMLDialogElement, event: { clientX: number; clientY: number }): boolean {
  const rect = dialog.getBoundingClientRect();
  return event.clientX < rect.left || event.clientX > rect.right
    || event.clientY < rect.top || event.clientY > rect.bottom;
}

document.addEventListener("pointerdown", (event) => {
  const target = event.target;
  backdropPress = target instanceof HTMLDialogElement && target.matches(DIALOG) && outside(target, event)
    ? target
    : null;
});

// open shows a dialog as a modal, described by its Description.
function open(dialog: HTMLDialogElement, invoker?: HTMLElement): void {
  const description = dialog.querySelector<HTMLElement>('[data-slot="dialog-description"]');
  if (description?.id) dialog.setAttribute("aria-describedby", description.id);
  if (!dialog.open) {
    settle();
    rememberOpening(dialog, invoker);
    dialog.showModal();
    restoreOrder(dialog);
  }
}

// A dialog drawn open (Props.Open) shows as soon as it is in the document:
// on the page it loaded with, or when a swap brings it in.
registerComponent(DIALOG, (dialog) => {
  if (!(dialog instanceof HTMLDialogElement)) return;
  const description = dialog.querySelector<HTMLElement>('[data-slot="dialog-description"]');
  if (description?.id) dialog.setAttribute("aria-describedby", description.id);
  if (dialog.hasAttribute("data-open-on-insert")) open(dialog);
});

document.addEventListener("click", (event) => {
  const target = event.target;
  if (!(target instanceof Element)) return;
  const backdrop = backdropPress;
  backdropPress = null;
  if (target === backdrop && !backdrop.hasAttribute("data-alert") && outside(backdrop, event)) {
    closeDialog(backdrop);
    return;
  }
  if (target.closest("button:disabled, [aria-disabled=true]")) return;
  const trigger = target.closest<HTMLElement>("[data-dialog-target]");
  if (trigger) {
    const dialog = document.getElementById(trigger.dataset.dialogTarget ?? "");
    if (dialog instanceof HTMLDialogElement && dialog.matches(DIALOG)) {
      event.preventDefault();
      const invoker = target.closest<HTMLElement>("button, a[href], [tabindex]")
        ?? trigger.querySelector<HTMLElement>("button, a[href], [tabindex]") ?? trigger;
      open(dialog, invoker);
    }
    return;
  }
  const close = target.closest('[data-slot="dialog-close"]');
  if (close) {
    const dialog = close.closest<HTMLDialogElement>(DIALOG);
    if (dialog) {
      event.preventDefault();
      closeDialog(dialog);
    }
  }
});
