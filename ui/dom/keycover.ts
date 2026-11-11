// The pure half of the router's projections in keyfocus.ts: which nodes lie
// outside a layer, how one attribute and its mark are written and released,
// and where focus goes once the node holding it is out of reach. Nothing
// here reads the DOM: a Tree says how to reach a node's parent and children,
// so keyfocus.ts passes the document and the tests pass plain objects.

export type Tree<T> = {
  children: (node: T) => readonly T[];
  parent: (node: T) => T | null;
};

// Marks reads and writes bare attributes on a node.
export type Marks<T> = {
  has: (node: T, name: string) => boolean;
  remove: (node: T, name: string) => void;
  set: (node: T, name: string) => void;
};

// covered lists the nodes outside shown, in document order: each sibling of
// a shown node or of one of its ancestors. It never enters a shown node or
// a listed one, and a quiet node -- a script, a template -- is neither
// listed nor entered. With nothing shown under root it lists nothing: a
// layer with no region leaves the page alone rather than covering all of it.
export function covered<T>(root: T, shown: Iterable<T>, tree: Tree<T>, quiet: (node: T) => boolean): T[] {
  const keep = new Set(shown);
  const path = new Set<T>();
  for (const node of keep) for (let at: T | null = node; at !== null; at = tree.parent(at)) path.add(at);
  if (!path.has(root)) return [];
  const out: T[] = [];
  const walk = (parent: T): void => {
    for (const child of tree.children(parent)) {
      if (keep.has(child) || quiet(child)) continue;
      if (path.has(child)) walk(child);
      else out.push(child);
    }
  };
  walk(root);
  return out;
}

// reconcile makes name present on exactly the wanted nodes the router owns.
// Each wanted node gets name and mark; a marked node outside wanted loses
// both; a node that has name without the mark is the author's and is left
// alone whether wanted or not. marked is every node carrying mark now, and
// a marked node that lost name gets it back. What is already there is never
// written again: in the DOM a write of the value in place still queues a
// mutation record, and the router settles on every batch, so a rewrite of
// its own inert would settle it without end.
export function reconcile<T>(
  name: string,
  mark: string,
  marked: Iterable<T>,
  wanted: Iterable<T>,
  marks: Marks<T>,
): void {
  const want = new Set(wanted);
  for (const node of marked) {
    if (want.has(node)) continue;
    marks.remove(node, mark);
    marks.remove(node, name);
  }
  for (const node of want) {
    if (marks.has(node, name) && !marks.has(node, mark)) continue;
    if (!marks.has(node, mark)) marks.set(node, mark);
    if (!marks.has(node, name)) marks.set(node, name);
  }
}

// Place is where a node sat while it was in the tree: its ancestors from the
// root down to the node itself, and each one's index among its parent's
// children, so a node a swap removed can still be placed against what is
// there now.
export type Place<T> = { at: number[]; chain: T[] };

export function place<T>(node: T, tree: Tree<T>): Place<T> {
  const chain: T[] = [];
  for (let at: T | null = node; at !== null; at = tree.parent(at)) chain.unshift(at);
  const at = chain.map((n, i) => (i === 0 ? 0 : tree.children(chain[i - 1]).indexOf(n)));
  return { at, chain };
}

// nearest picks, among candidates in document order, the one closest to
// where lost sat: the deepest ancestor the two share decides, and among
// candidates under the same ancestor the first one at or past lost's branch
// wins, else the last one before it. A candidate inside lost itself is
// never picked, and one sharing no ancestor is not in the tree.
export function nearest<T>(lost: Place<T>, candidates: readonly T[], tree: Tree<T>): T | null {
  let best: { after: boolean; depth: number; node: T; order: number } | null = null;
  for (let order = 0; order < candidates.length; order++) {
    const node = candidates[order];
    const up: T[] = [];
    for (let at: T | null = node; at !== null; at = tree.parent(at)) up.unshift(at);
    let depth = -1;
    while (depth + 1 < lost.chain.length && depth + 1 < up.length && lost.chain[depth + 1] === up[depth + 1]) depth++;
    if (depth < 0 || depth === lost.chain.length - 1) continue;
    const branch = tree.children(lost.chain[depth]).indexOf(up[depth + 1]);
    const after = branch >= lost.at[depth + 1];
    const better = !best
      || depth > best.depth
      || (depth === best.depth && (after ? !best.after || order < best.order : !best.after && order > best.order));
    if (better) best = { after, depth, node, order };
  }
  return best ? best.node : null;
}
