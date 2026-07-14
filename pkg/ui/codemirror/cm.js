import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, drawSelection, highlightActiveLine, highlightActiveLineGutter, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { StreamLanguage, bracketMatching, defaultHighlightStyle, foldGutter, foldKeymap, indentOnInput, syntaxHighlighting } from "@codemirror/language";
import { acceptCompletion, autocompletion, completionKeymap } from "@codemirror/autocomplete";
import { searchKeymap } from "@codemirror/search";
import { SQLite, sql } from "@codemirror/lang-sql";
import { go } from "@codemirror/lang-go";
import { javascript } from "@codemirror/lang-javascript";
import { html } from "@codemirror/lang-html";
import { css } from "@codemirror/lang-css";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { python } from "@codemirror/lang-python";
import { xml } from "@codemirror/lang-xml";
import { yaml } from "@codemirror/lang-yaml";
import { shell } from "@codemirror/legacy-modes/mode/shell";
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

function languageFor(filename) {
  const ext = (filename.split(".").pop() || "").toLowerCase();
  switch (ext) {
    case "go":
      return go();
    case "js":
    case "mjs":
    case "cjs":
      return javascript();
    case "jsx":
      return javascript({ jsx: true });
    case "ts":
      return javascript({ typescript: true });
    case "tsx":
      return javascript({ jsx: true, typescript: true });
    case "html":
    case "htm":
    case "templ":
      return html();
    case "css":
      return css();
    case "json":
      return json();
    case "md":
    case "markdown":
      return markdown();
    case "py":
      return python();
    case "svg":
    case "xml":
      return xml();
    case "yaml":
    case "yml":
      return yaml();
    case "bash":
    case "sh":
      return StreamLanguage.define(shell);
    case "sql":
      return sql({ dialect: SQLite });
    default:
      return [];
  }
}

window.CMFILES = {
  create(parent, onChange) {
    const language = new Compartment();
    const historyC = new Compartment();
    const view = new EditorView({
      parent,
      state: EditorState.create({
        doc: "",
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          drawSelection(),
          foldGutter(),
          historyC.of(history()),
          indentOnInput(),
          bracketMatching(),
          syntaxHighlighting(defaultHighlightStyle),
          EditorView.lineWrapping,
          EditorView.theme({ "&": { height: "100%", fontSize: "13px" }, ".cm-scroller": { overflow: "auto" } }),
          keymap.of([indentWithTab, ...defaultKeymap, ...historyKeymap, ...foldKeymap, ...searchKeymap]),
          language.of([]),
          EditorView.updateListener.of((u) => {
            if (u.docChanged && onChange) onChange();
          }),
        ],
      }),
    });
    return {
      focus: () => view.focus(),
      getValue: () => view.state.doc.toString(),
      load: (filename, text) => {
        view.dispatch({
          changes: { from: 0, to: view.state.doc.length, insert: text },
          effects: [language.reconfigure(languageFor(filename)), historyC.reconfigure(history())],
        });
      },
      view,
    };
  },
};
