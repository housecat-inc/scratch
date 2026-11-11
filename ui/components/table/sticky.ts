import { registerComponent } from "../../dom/lifecycle";

// Category rows use the standard h-10 column header height before JavaScript
// loads. Native sticky pins them; the next category supplies their push-away
// offset because table rows all share the table's containing block.
const HEADER = "thead[data-sticky-header]";
const observed = new Set<HTMLTableElement>();
const offsets = new WeakMap<HTMLElement, number>();
let frame = 0;
const dirty = new Set<HTMLTableElement>();

function updateCategories(table: HTMLTableElement): void {
  const rows = Array.from(table.querySelectorAll<HTMLElement>("[data-sticky-row]")).filter((row) =>
    row.closest("table") === table && row.getClientRects().length > 0
  );
  const boxes = rows.map((row) => {
    const box = row.getBoundingClientRect();
    return { top: box.top - (offsets.get(row) ?? 0), height: box.height };
  });
  // Read first, then write. Removing our previous translation from each
  // rectangle keeps the native sticky position as the stable reference.
  rows.forEach((row, i) => {
    const box = boxes[i];
    const next = boxes[i + 1] as typeof box | undefined;
    const offset = next ? Math.max(-box.height, Math.min(0, next.top - box.top - box.height)) : 0;
    if (offsets.get(row) === offset) return;
    offsets.set(row, offset);
    row.style.setProperty("--table-sticky-offset", `${offset}px`);
  });
}

function schedule(table?: HTMLTableElement): void {
  if (table) dirty.add(table);
  else for (const table of observed) dirty.add(table);
  if (frame || !dirty.size) return;
  frame = requestAnimationFrame(() => {
    frame = 0;
    for (const table of dirty) if (table.isConnected && observed.has(table)) updateCategories(table);
    dirty.clear();
  });
}

registerComponent("table", (table, signal) => {
  if (!(table instanceof HTMLTableElement)) return;
  let header: HTMLTableSectionElement | null = null;
  const headerSize = () => table.closest<HTMLElement>('[data-slot="table-frame"]') ?? table;
  const resize = new ResizeObserver(() => {
    if (!header || signal.aborted) return;
    const height = `${header.getBoundingClientRect().height}px`;
    const owner = headerSize();
    if (owner.style.getPropertyValue("--table-header-height") !== height) {
      owner.style.setProperty("--table-header-height", height);
    }
    schedule(table);
  });
  const reconcile = () => {
    const next = Array.from(table.querySelectorAll<HTMLTableSectionElement>(HEADER))
      .find(candidate => candidate.closest("table") === table) ?? null;
    if (header !== next) {
      resize.disconnect();
      header = next;
      if (header) {
        observed.add(table);
        resize.observe(header);
      } else {
        observed.delete(table);
        headerSize().style.removeProperty("--table-header-height");
      }
    }
    if (header) schedule(table);
  };
  const mutations = new MutationObserver(reconcile);
  mutations.observe(table, {
    childList: true,
    subtree: true,
    attributeFilter: ["hidden", "data-sticky-header", "data-sticky-row"],
  });
  signal.addEventListener("abort", () => {
    mutations.disconnect();
    resize.disconnect();
    observed.delete(table);
    dirty.delete(table);
    if (!dirty.size) {
      cancelAnimationFrame(frame);
      frame = 0;
    }
  }, { once: true });
  reconcile();
});

document.addEventListener("scroll", () => schedule(), { capture: true, passive: true });
