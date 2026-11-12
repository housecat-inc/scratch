// Every history entry Housecat writes is one htmx can restore. htmx restores
// a traversal only when history.state.htmx is truthy, the marker its own
// pushes and replaces carry, so both writers here set it: push on the new
// entry, replace on the entry it rewrites, whatever that entry held, so an
// entry written before htmx marked the page load is as restorable as one
// written after. replace applies the patch over the entry's state; push
// starts from the patch alone. Inline scripts cannot import, so the seam is
// window.HousecatHistory, as HousecatKeys and HousecatSegmented are.

import * as Predicate from "effect/Predicate";

export type Patch = Record<string, unknown>;

// merged is base with patch applied: a key the patch sets to undefined is
// deleted, so an entry can be rewritten without a key it has no use for.
export function merged(base: unknown, patch: Patch): Record<string, unknown> {
  const out: Record<string, unknown> = Predicate.isObject(base) ? { ...base } : {};
  for (const [key, value] of Object.entries(patch)) {
    if (value === undefined) delete out[key];
    else out[key] = value;
  }
  return out;
}

export function push(patch: Patch, url: string | URL): void {
  history.pushState({ ...merged({}, patch), htmx: true }, "", url);
}

export function replace(patch: Patch, url: string | URL): void {
  history.replaceState({ ...merged(history.state, patch), htmx: true }, "", url);
}

declare global {
  interface Window {
    HousecatHistory: { push: typeof push; replace: typeof replace };
  }
}

if (typeof window !== "undefined") window.HousecatHistory = { push, replace };
