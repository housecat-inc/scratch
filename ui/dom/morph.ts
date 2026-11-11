// A morph draws the server's copy of a subtree over the one on the page and
// keeps the nodes the two share. What the page's scripts have drawn on those
// nodes, and the server never renders, would be wiped by it, so a morph runs
// under a MorphRule: ignore names the attributes that the server's copy
// neither sets nor removes on an element that carries them, each by its
// whole name, so an attribute whose name only begins with an ignored one
// still morphs, and an element the morph builds afresh, which carries
// nothing yet, takes the server's; carry copies what the
// page drew onto the server's copy of an element before htmx compares the
// two. A rule holds only inside the element its morph swaps and only while
// that swap runs, through one htmx extension, so a morph elsewhere on the
// page is untouched; htmx.config.morphIgnore, which every morph reads, is
// left alone.

import { isElement } from "./dom";

export type MorphRule = {
  carry?: (from: Element, to: Element) => void;
  ignore: readonly string[];
};

export type MorphExtension = {
  htmx_before_morph_attr: (el: Element, detail: { attrName: string; newValue: string | null }) => boolean;
  htmx_before_morph_node: (el: Node, detail: { newNode: Node; oldNode: Node }) => boolean;
};

export type MorphHtmx = {
  registerExtension: (name: string, extension: MorphExtension) => void;
  swap: (options: { sourceElement: Element; swap: string; target: Element; text: string }) => Promise<void>;
};

// The rules of the morphs under way, by the element each one swaps.
const active = new Map<Element, MorphRule>();

function ruleFor(el: Node): MorphRule | null {
  for (const [target, rule] of active) if (target.contains(el)) return rule;
  return null;
}

// htmx calls these for each pair of elements its morph matches: node before
// it compares them, attr before it writes or removes each attribute that
// differs. Text and comment nodes carry nothing.
export const extension: MorphExtension = {
  htmx_before_morph_attr(el, detail) {
    const rule = ruleFor(el);
    return !rule || !rule.ignore.includes(detail.attrName) || !el.hasAttribute(detail.attrName);
  },
  htmx_before_morph_node(el, detail) {
    if (!isElement(el) || !isElement(detail.newNode)) return true;
    ruleFor(el)?.carry?.(el, detail.newNode);
    return true;
  },
};

// morph swaps html into target's children by htmx's innerMorph, with rule in
// force inside target until the swap settles or fails.
export function morph(htmx: MorphHtmx, target: Element, html: string, rule: MorphRule): Promise<void> {
  active.set(target, rule);
  const end = () => {
    if (active.get(target) === rule) active.delete(target);
  };
  return htmx.swap({ sourceElement: target, swap: "innerMorph", target, text: html }).then(end, (error: unknown) => {
    end();
    throw error;
  });
}

export function install(htmx: Pick<MorphHtmx, "registerExtension">): void {
  htmx.registerExtension("housecat-morph", extension);
}

// Node, where the tests run, has no window at all, so it is checked by typeof.
const pageHtmx = typeof window === "undefined" ? undefined : window.htmx;
if (pageHtmx) install(pageHtmx);
