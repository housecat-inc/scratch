import { registerComponent } from "../../dom/lifecycle";

const ROOT = "[data-scroll-fade]";
const FOOTER = "[data-scroll-fade-part=footer]";
const NAVIGATION = "[data-slot=bottombar]";
const PARTS = `${ROOT}, ${FOOTER}, ${NAVIGATION}`;

function outerHeight(element: HTMLElement | null): number {
  if (!element || element.getClientRects().length === 0) return 0;
  const style = getComputedStyle(element);
  return element.getBoundingClientRect().height + (parseFloat(style.marginTop) || 0)
    + (parseFloat(style.marginBottom) || 0);
}

function publish(element: HTMLElement, property: string, height: number): void {
  const value = `${Math.max(0, Math.ceil(height))}px`;
  if (element.style.getPropertyValue(property) !== value) element.style.setProperty(property, value);
}

function install(): void {
  const roots = new Set<HTMLElement>();
  let navigation: HTMLElement | null = null;
  let shell: HTMLElement | null = null;

  function measure(): void {
    if (shell) publish(shell, "--bottom-navigation-height", outerHeight(navigation));
    for (const root of roots) {
      const footer = root.querySelector<HTMLElement>(`${FOOTER}:not([data-compose-active])`);
      const scroller = root.querySelector<HTMLElement>("[data-chat-scroll]");
      const follow = scroller?.dataset.follow === "true";
      publish(root, "--scroll-footer-height", outerHeight(footer));
      if (scroller && follow) {
        scroller.scrollTop = scroller.scrollHeight;
        scroller.dataset.scrollTop = String(scroller.scrollTop);
      }
    }
  }

  const sizes = new ResizeObserver(measure);
  const states = new MutationObserver(measure);

  function discover(): void {
    roots.clear();
    sizes.disconnect();
    states.disconnect();
    navigation = document.querySelector<HTMLElement>(NAVIGATION);
    shell = navigation?.closest<HTMLElement>(".mail-shell") ?? null;
    if (navigation) sizes.observe(navigation);
    for (const root of document.querySelectorAll<HTMLElement>(ROOT)) {
      roots.add(root);
      const footer = root.querySelector<HTMLElement>(FOOTER);
      if (footer) {
        sizes.observe(footer);
        states.observe(footer, {
          attributes: true,
          attributeFilter: ["data-compose-active", "data-more-tools-active", "data-infer-tools-active", "hidden"],
        });
      }
    }
    measure();
  }

  registerComponent(PARTS, () => {
    discover();
    return discover;
  });
  window.addEventListener("resize", measure, { passive: true });
  discover();
}

if (typeof document !== "undefined") install();
