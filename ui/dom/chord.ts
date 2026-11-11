// Chords as bindings write them -- "Escape", "mod+k", "ctrl+k", "shift+Tab",
// "," -- and whether a key press is one. Modifiers match exactly. mod is
// Command on a Mac and Control elsewhere, read from html[data-platform] on
// every press. A letter matches the key it types, case aside, and its
// physical key only when the layout types no ASCII letter there: Command-K
// works on a Cyrillic layout, and Dvorak's key under QWERTY's K, which types
// "t", never fires mod+k.

import { isMac } from "./keyinfo";

export type Chord = {
  alt: boolean;
  ctrl: boolean;
  key: string;
  meta: boolean;
  mod: boolean;
  shift: boolean;
};

const MODIFIER_NAMES = ["alt", "ctrl", "meta", "mod", "shift"] as const;
const BARE = new Set(["Alt", "AltGraph", "Control", "Meta", "Shift"]);
const LETTER = /^[a-z]$/i;
const ASCII = /^[\x20-\x7e]$/;
const FUNCTION_KEY = /^F([1-9]|1[0-2])$/;

export function parseChord(text: string): Chord {
  const plus = text === "+" || text.endsWith("++");
  const parts = (plus ? text.slice(0, -1) : text).split("+").filter((part) => part !== "");
  const key = plus ? "+" : (parts.pop() ?? "");
  // A chord needs a key besides its modifiers: "mod+" or "ctrl+shift" names
  // none, and a modifier pressed on its own never reaches a binding.
  if (key === "" || (MODIFIER_NAMES as readonly string[]).includes(key.toLowerCase()) || BARE.has(key)) {
    throw new TypeError("HousecatKeys: " + JSON.stringify(text) + " names no key to bind");
  }
  const mods = new Set(parts.map((part) => part.toLowerCase()));
  for (const mod of mods) {
    if (!(MODIFIER_NAMES as readonly string[]).includes(mod)) {
      throw new TypeError("HousecatKeys: unknown modifier " + JSON.stringify(mod) + " in " + JSON.stringify(text));
    }
  }
  return {
    alt: mods.has("alt"),
    ctrl: mods.has("ctrl"),
    key: key.toLowerCase() === "space" ? " " : key,
    meta: mods.has("meta"),
    mod: mods.has("mod"),
    shift: mods.has("shift"),
  };
}

// chordName writes a chord the one way bindings() reports it.
export function chordName(chord: Chord): string {
  const key = chord.key === " " ? "space" : LETTER.test(chord.key) ? chord.key.toLowerCase() : chord.key;
  return [...MODIFIER_NAMES.filter((name) => chord[name]), key].join("+");
}

function keyMatches(key: string, event: KeyboardEvent): boolean {
  if (!LETTER.test(key)) return event.key === key;
  // AltGr types a character of its own -- Polish "ą" for AltGr+A, but also an
  // ASCII one on some layouts -- so the physical key it sits on names no
  // letter chord; checked first, ahead of the ASCII match below, so an ASCII
  // AltGr character never fires a binding by coinciding with its letter
  // either. Without this, a ctrl+alt+a binding would fire for AltGr+A on a
  // non-field target, since Windows reports AltGr as Control and Alt both
  // held.
  if (altGraph(event)) return false;
  if (ASCII.test(event.key)) return event.key.toLowerCase() === key.toLowerCase();
  return event.code === "Key" + key.toUpperCase();
}

export function matchChord(chord: Chord, event: KeyboardEvent): boolean {
  const mac = isMac();
  if (event.ctrlKey !== (chord.ctrl || (chord.mod && !mac))) return false;
  if (event.metaKey !== (chord.meta || (chord.mod && mac))) return false;
  if (event.altKey !== chord.alt || event.shiftKey !== chord.shift) return false;
  return keyMatches(chord.key, event);
}

// The chords a text field answers -- select all, copy, paste, cut, undo, redo
// -- and the browser's own zoom, find, reload and developer tools. Neither is
// ever delivered to a binding.
const EDITING = ["mod+a", "mod+c", "mod+v", "mod+shift+v", "mod+x", "mod+y", "mod+z", "mod+shift+z"].map(parseChord);
const RESERVED = [
  "mod+=",
  "mod++",
  "mod+shift+=",
  "mod+shift++",
  "mod+-",
  "mod+shift+-",
  "mod+shift+_",
  "mod+0",
  "mod+f",
  "mod+g",
  "mod+shift+g",
  "mod+r",
  "mod+shift+r",
  "F5",
  "F11",
  "F12",
  "mod+alt+c",
  "mod+alt+i",
  "mod+alt+j",
  "mod+shift+c",
  "mod+shift+i",
  "mod+shift+j",
].map(parseChord);

export function editingChord(event: KeyboardEvent): boolean {
  return EDITING.some((chord) => matchChord(chord, event));
}

export function reservedChord(event: KeyboardEvent): boolean {
  return RESERVED.some((chord) => matchChord(chord, event));
}

// altGraph is a key typed with AltGr, which Windows reports with Control and
// Alt both held: "@", "€" or "{" on many layouts. It types text and is never
// a chord.
export function altGraph(event: KeyboardEvent): boolean {
  return event.getModifierState("AltGraph");
}

// bareModifier is a modifier key pressed on its own, which no binding takes.
export function bareModifier(event: KeyboardEvent): boolean {
  return BARE.has(event.key);
}

// browserShortcut is whether the browser, rather than the page, may answer
// event: any key with Command, Control or Alt -- find, zoom, reload, back --
// but AltGr text, and the function keys. A key the router keeps from the page
// outside a modal layer is still left to the browser when it is one.
export function browserShortcut(event: KeyboardEvent): boolean {
  if (FUNCTION_KEY.test(event.key)) return true;
  return (event.metaKey || event.ctrlKey || event.altKey) && !altGraph(event);
}
