import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { defaultHighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { SQLite, sql } from "@codemirror/lang-sql";
import { acceptCompletion, autocompletion, completionKeymap } from "@codemirror/autocomplete";
import { languageServer } from "codemirror-languageserver";
import { createFromBuffer } from "@dprint/formatter";

window.CMSQL = {
  Compartment,
  EditorState,
  EditorView,
  SQLite,
  acceptCompletion,
  autocompletion,
  completionKeymap,
  createFromBuffer,
  defaultHighlightStyle,
  defaultKeymap,
  history,
  historyKeymap,
  keymap,
  languageServer,
  lineNumbers,
  sql,
  syntaxHighlighting,
};
