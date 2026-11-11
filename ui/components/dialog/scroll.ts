import { registerComponent } from "../../dom/lifecycle";

const DIALOG = 'dialog[data-slot="dialog"]';
let unlock: (() => void) | undefined;

function sync(): void {
  if (!document.querySelector(DIALOG + ":modal")) {
    unlock?.();
    unlock = undefined;
    return;
  }
  if (unlock) return;
  const page = document.documentElement;
  const classes = ["overflow-hidden"];
  if (window.innerWidth > page.clientWidth) classes.push("[scrollbar-gutter:stable]");
  const added = classes.filter(name => !page.classList.contains(name));
  page.classList.add(...added);
  unlock = () => page.classList.remove(...added);
}

registerComponent(DIALOG, (root, signal) => {
  const observer = new MutationObserver(sync);
  observer.observe(root, { attributeFilter: ["open"] });
  signal.addEventListener("abort", () => {
    observer.disconnect();
    sync();
  }, { once: true });
  sync();
});
