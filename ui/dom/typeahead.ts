// The typeahead a list of options or items shares: a buffer its printable keys
// spell into, and the search that finds the item the buffer names.
//
// The rules are a native <select>'s, measured rather than guessed: one key
// pressed over and over walks the items answering it, anything else is a
// prefix of the whole item, the search always starts after the item the
// cursor is on, and a query nothing answers moves nowhere. Matching a word
// inside an item, or holding still on the one already under the cursor, would
// both read as a list behaving unlike every other select.

export type Typeahead = {
  // clear drops the buffer. A script keeps one buffer for every list on the
  // page, since only one is open at a time, so it clears it whenever its list
  // closes: a list left open and closed inside the reset window would
  // otherwise hand its leftovers to the next one, whose first press would read
  // as a second.
  clear: () => void;
  // next adds key to the query and returns the item among items the query
  // names, or null when nothing answers. cursor is the item the cursor is on,
  // and textOf reads the text an item answers to.
  next: (
    items: HTMLElement[],
    key: string,
    cursor: HTMLElement | null,
    textOf: (item: HTMLElement) => string,
  ) => HTMLElement | null;
};

// isTypeaheadKey answers whether a key spells rather than acts. Space is not
// one: every list that types also spends Space on the item under the cursor --
// a menu activates it, a combobox commits it -- and a key cannot both carry a
// query and choose. Letting it do either leaves Space dead for a whole
// TYPEAHEAD_RESET after any letter, which is measurable: in Chrome, with Space
// in the query, r then Space chose nothing at 0, 200, 400, 600, 800 or 900ms.
// Items that need a space to tell them apart -- "Sign in" against "Sign out"
// -- want clearer names.
export function isTypeaheadKey(event: KeyboardEvent): boolean {
  return event.key.length === 1 && event.key !== " " && !event.ctrlKey && !event.metaKey && !event.altKey;
}

// TYPEAHEAD_RESET is the pause between keys that starts a new query, and it is
// one number for every list on the page: a menu and a select that dropped
// their buffers at different times would be the same gesture behaving two
// ways. Measured against a native <select>, which keeps its buffer through an
// 800ms pause and has dropped it by 1000ms.
//
// The Go browser tests cannot import this, so each one keeps a `typeaheadReset`
// of its own that has to outlast it. Change this and grep TYPEAHEAD_RESET.
export const TYPEAHEAD_RESET = 1000;

// typeahead makes a buffer that starts a new query after a TYPEAHEAD_RESET
// pause between keys.
export function typeahead(): Typeahead {
  const reset = TYPEAHEAD_RESET;
  let typed = "";
  let typedAt = 0;
  // The item the cursor sat on when the current query began. A query being
  // spelled searches from here rather than from wherever the last keystroke
  // left the cursor: every prefix of a word the items share matches them all,
  // so searching from the cursor would step to the next one on every letter
  // instead of narrowing the same search.
  let typedFrom: HTMLElement | null = null;

  return {
    clear() {
      typed = "";
      typedAt = 0;
      typedFrom = null;
    },
    next(items, key, cursor, textOf) {
      // Monotonic, so a clock stepped backwards cannot leave the buffer never
      // resetting and fold a lone press into the one before it.
      const now = performance.now();
      const fresh = now - typedAt > reset;
      typed = fresh ? key : typed + key;
      typedAt = now;
      if (fresh) typedFrom = cursor;
      const query = typed.toLowerCase();
      const held = heldKey(query);
      // A key held down walks from where the cursor is now; anything spelled
      // searches from where the spelling started.
      const base = held ? cursor : typedFrom;
      const start = base ? items.indexOf(base) : -1;
      const ordered = items.slice(start + 1).concat(items.slice(0, start + 1));
      const answers = (prefix: string) => (item: HTMLElement) => textOf(item).trim().toLowerCase().startsWith(prefix);
      if (held) {
        const next = ordered.find(answers(held));
        if (next) return next;
      }
      return ordered.find(answers(query)) ?? null;
    },
  };
}

// heldKey is the key a query spells by being pressed over and over, or "" for
// anything else. It reads the buffer rather than splitting it, since a key held
// down appends on every repeat and would make the check quadratic.
function heldKey(query: string): string {
  for (let i = 1; i < query.length; i += 1) {
    if (query[i] !== query[0]) return "";
  }
  return query.charAt(0);
}
