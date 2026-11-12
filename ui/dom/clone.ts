export function cloneElement<T extends HTMLElement>(source: T): T;
export function cloneElement(source: Element): HTMLElement;
export function cloneElement(source: Element): HTMLElement {
  const node = source.cloneNode(true);
  if (!(node instanceof HTMLElement)) throw new TypeError("Expected an HTML template element");
  return node;
}

export function cloneNode<T extends Node>(source: T): T {
  // oxlint-disable-next-line typescript/no-unsafe-type-assertion -- cloneNode preserves the concrete node type
  return source.cloneNode(true) as T;
}
