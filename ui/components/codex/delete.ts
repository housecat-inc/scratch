// Escape in a chat's delete confirm is the confirm's Cancel button. It is a
// router binding on the confirm, so it runs before the Escape of whatever
// holds the confirm -- the mail chat rail is a modal drawer at narrow widths
// -- and one press never both cancels the delete and closes the panel. A
// confirm with no Cancel leaves Escape to what holds it.

import { on } from "../../dom/keys";

on(
  "[data-chat-delete-confirm]",
  {
    Escape: (_event, confirm) => {
      const cancel = confirm.querySelector<HTMLElement>("[data-chat-delete-cancel]");
      cancel?.click();
      return !!cancel;
    },
  },
  { typing: "chords" },
);
