import { registerComponent } from "./lifecycle";

registerComponent("[data-stop-click]", (root, signal) => {
  root.addEventListener("click", (event) => event.stopPropagation(), { signal });
});

registerComponent("textarea[data-caret-end]", (root, signal) => {
  if (!(root instanceof HTMLTextAreaElement)) return;
  const move = () => root.setSelectionRange(root.value.length, root.value.length);
  root.addEventListener("focus", move, { signal });
  if (document.activeElement === root) move();
});

registerComponent('[data-action="history-back"]', (root, signal) => {
  root.addEventListener(
    "click",
    () => {
      if (history.length > 1) history.back();
      else location.href = "/home";
    },
    { signal },
  );
});

registerComponent("form[data-prevent-native-submit]", (root, signal) => {
  root.addEventListener("submit", (event) => event.preventDefault(), { signal });
});

registerComponent('[data-action="reload"]', (root, signal) => {
  root.addEventListener("click", () => location.reload(), { signal });
});

registerComponent('[data-action="hide-support-debug"]', (root, signal) => {
  root.addEventListener("click", () => document.body.removeAttribute("data-support-debug"), { signal });
});

registerComponent("[data-copy-value]:not([data-copy-chip])", (root, signal) => {
  if (!(root instanceof HTMLElement)) return;
  root.addEventListener("click", () => void navigator.clipboard.writeText(root.dataset.copyValue ?? ""), { signal });
});

registerComponent("[data-copy-from]", (root, signal) => {
  if (!(root instanceof HTMLElement)) return;
  root.addEventListener(
    "click",
    () => {
      const source = document.getElementById(root.dataset.copyFrom ?? "");
      if (source) void navigator.clipboard.writeText(source.textContent);
    },
    { signal },
  );
});

registerComponent("[data-copy-chip]", (root, signal) => {
  if (!(root instanceof HTMLElement)) return;
  let timer = 0;
  root.addEventListener(
    "click",
    (event) => {
      event.stopPropagation();
      void navigator.clipboard.writeText(root.dataset.copyValue ?? "").then(() => {
        root.dataset.copied = "true";
        clearTimeout(timer);
        timer = window.setTimeout(() => delete root.dataset.copied, 1500);
      });
    },
    { signal },
  );
});
