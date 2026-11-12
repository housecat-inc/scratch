import { setAttributeIfChanged } from "../../dom/dom";
import { registerComponent } from "../../dom/lifecycle";

registerComponent("[data-chat-scroll]", (root, signal) => {
  const follow = () => {
    if (signal.aborted || root.dataset.follow !== "true") return;
    root.scrollTop = root.scrollHeight;
    setAttributeIfChanged(root, "data-scroll-top", String(root.scrollTop));
  };
  if (root.dataset.initialized !== "true") {
    root.scrollTop = root.scrollHeight;
    setAttributeIfChanged(root, "data-scroll-top", String(root.scrollTop));
    setAttributeIfChanged(root, "data-initialized", "true");
  }
  const resize = new ResizeObserver(follow);
  resize.observe(root);
  signal.addEventListener("abort", () => resize.disconnect(), { once: true });
});
