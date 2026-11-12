type Dispose = () => void;
type Mount = (root: HTMLElement, signal: AbortSignal) => Dispose | void;
type Instance = { controller: AbortController; dispose: Dispose | void };
type Registration = { selector: string; mount: Mount; roots: Map<HTMLElement, Instance> };

const registrations = new Set<Registration>();
let observer: MutationObserver | undefined;

function rootsIn(node: Element, selector: string): HTMLElement[] {
  const roots = Array.from(node.querySelectorAll<HTMLElement>(selector));
  if (node instanceof HTMLElement && node.matches(selector)) roots.unshift(node);
  return roots;
}

function connected(root: HTMLElement): boolean {
  return root.isConnected && root.ownerDocument === document;
}

function dispose(registration: Registration, root: HTMLElement): void {
  const instance = registration.roots.get(root);
  if (!instance) return;
  registration.roots.delete(root);
  instance.controller.abort();
  try {
    instance.dispose?.();
  } catch (error) {
    reportError(error);
  }
}

function discover(registration: Registration, subtree: Element): void {
  for (const root of rootsIn(subtree, registration.selector)) {
    if (!connected(root) || registration.roots.has(root) || !registrations.has(registration)) continue;
    const instance: Instance = { controller: new AbortController(), dispose: undefined };
    registration.roots.set(root, instance);
    try {
      instance.dispose = registration.mount(root, instance.controller.signal);
    } catch (error) {
      dispose(registration, root);
      reportError(error);
    }
  }
}

// Discovery mounts new roots only. Component state updates belong to explicit
// events or root-local observers, so discovering a subtree never renders it again.
export function discoverComponents(subtree: Element): void {
  for (const registration of registrations) discover(registration, subtree);
}

// Selectors identify stable roots; attribute changes are component inputs rather
// than discovery. A connected move within this document keeps the same instance.
export function registerComponent(selector: string, mount: Mount): Dispose {
  document.documentElement.matches(selector);
  const registration: Registration = { selector, mount, roots: new Map() };
  registrations.add(registration);
  if (!observer) {
    observer = new MutationObserver(changes => {
      const added = new Set<Element>();
      const removed = new Set<Element>();
      for (const change of changes) {
        for (const node of change.addedNodes) if (node instanceof Element) added.add(node);
        for (const node of change.removedNodes) if (node instanceof Element) removed.add(node);
      }
      for (const subtree of added) discoverComponents(subtree);
      for (const registration of registrations) {
        for (const subtree of removed) {
          for (const root of rootsIn(subtree, registration.selector)) {
            if (!connected(root)) dispose(registration, root);
          }
        }
      }
    });
    observer.observe(document.documentElement, { childList: true, subtree: true });
  }
  discover(registration, document.documentElement);
  return () => {
    registrations.delete(registration);
    for (const root of registration.roots.keys()) dispose(registration, root);
    if (!registrations.size) {
      observer?.disconnect();
      observer = undefined;
    }
  };
}
