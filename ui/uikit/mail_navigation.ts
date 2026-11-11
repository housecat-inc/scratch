import { registerComponent } from "../dom/lifecycle";

function outline(list: Element): void {
  list.querySelectorAll('[data-slot="mail-row"]').forEach((row) =>
    row.classList.toggle("focused", row.closest('[data-collection="row"]')?.hasAttribute("data-current") === true)
  );
}

registerComponent('[data-slot="mail-list"]', (list) => outline(list));

document.addEventListener("collection:current-change", (event) => {
  const root = event.target;
  if (root instanceof Element && root.matches('[data-slot="mail-list"]')) outline(root);
});

// A row carries an id, so an innerHTML swap's settle step gives the new row
// the attributes of the row it replaced and, after the settle delay, puts the
// server's back, class included. A row made current in between loses its
// outline then, so each settle draws the outline again from data-current.
document.addEventListener("htmx:after:settle", (event) => {
  const target = event.target;
  if (!(target instanceof Element)) return;
  if (target.matches('[data-slot="mail-list"]')) outline(target);
  target.querySelectorAll('[data-slot="mail-list"]').forEach(outline);
});
