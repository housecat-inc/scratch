import { registerComponent } from "../../dom/lifecycle";

registerComponent("form[data-prompt-max-height]", (root, signal) => {
  const field = root.querySelector<HTMLTextAreaElement>("[data-prompt-field]");
  if (!field) return undefined;
  let width = -1;
  let font = "";

  function resize(): void {
    if (!field || !root.isConnected) return;
    width = root.getBoundingClientRect().width;
    font = getComputedStyle(field).font;
    field.style.height = "auto";
    const cap = Number(root.dataset.promptMaxHeight) - parseFloat(getComputedStyle(field).marginBottom);
    field.style.height = `${Math.min(field.scrollHeight, cap)}px`;
  }

  const observer = new ResizeObserver(() => {
    if (width !== root.getBoundingClientRect().width || font !== getComputedStyle(field).font) resize();
  });
  observer.observe(root);
  root.addEventListener("input", (event) => {
    if (event.target === field) resize();
  }, { signal });
  window.addEventListener("resize", resize, { signal, passive: true });
  void document.fonts.ready.then(() => {
    if (!signal.aborted) resize();
  });
  resize();
  return () => observer.disconnect();
});
