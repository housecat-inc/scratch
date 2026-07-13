import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { SQLite, sql } from "@codemirror/lang-sql";
import { acceptCompletion, autocompletion, completionKeymap } from "@codemirror/autocomplete";
import { languageServer } from "codemirror-languageserver";

window.CMSQL = {
  Compartment,
  EditorState,
  EditorView,
  SQLite,
  acceptCompletion,
  autocompletion,
  completionKeymap,
  defaultKeymap,
  history,
  historyKeymap,
  keymap,
  languageServer,
  lineNumbers,
  sql,
};
