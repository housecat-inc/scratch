import { Compartment, EditorState } from "https://esm.sh/@codemirror/state@6.5.2";
import { EditorView, keymap, lineNumbers } from "https://esm.sh/@codemirror/view@6.38.1?deps=@codemirror/state@6.5.2";
import { defaultKeymap, history, historyKeymap } from "https://esm.sh/@codemirror/commands@6.6.0?deps=@codemirror/state@6.5.2,@codemirror/view@6.38.1";
import { SQLite, sql } from "https://esm.sh/@codemirror/lang-sql@6.8.0?deps=@codemirror/state@6.5.2,@codemirror/view@6.38.1";
import { autocompletion, completionKeymap } from "https://esm.sh/@codemirror/autocomplete@6.18.6?deps=@codemirror/state@6.5.2,@codemirror/view@6.38.1";

const mount = document.getElementById("sql-cm");
const ta = document.getElementById("sql-editor");
if (mount && ta) {
  const sqlCompartment = new Compartment();
  const lspCompartment = new Compartment();

  const sync = EditorView.updateListener.of((update) => {
    if (update.docChanged) ta.value = update.state.doc.toString();
  });

  const heightTheme = EditorView.theme({
    "&": { height: "180px", fontSize: "13px" },
    ".cm-scroller": { overflow: "auto" },
  });

  function runForm() {
    const form = document.getElementById("sql-run-form");
    if (form && window.htmx) window.htmx.trigger(form, "submit");
    return true;
  }

  const view = new EditorView({
    state: EditorState.create({
      doc: ta.value,
      extensions: [
        lineNumbers(),
        history(),
        keymap.of([{ key: "Mod-Enter", run: runForm }]),
        keymap.of([...defaultKeymap, ...historyKeymap, ...completionKeymap]),
        sqlCompartment.of(sql({ dialect: SQLite })),
        autocompletion(),
        heightTheme,
        sync,
        lspCompartment.of([]),
      ],
    }),
    parent: mount,
  });

  ta.classList.add("hidden");
  ta.value = view.state.doc.toString();

  window.sqlEditor = view;
  window.sqlGetEditor = () => view.state.doc.toString();
  window.sqlSetEditor = (text) => {
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text || "" } });
  };

  const path = (window.sqlPath && window.sqlPath()) || "";
  const query = path ? "?path=" + encodeURIComponent(path) : "";

  (async () => {
    try {
      const res = await fetch("/sql/schema" + query, { credentials: "same-origin" });
      if (!res.ok) return;
      const { tables } = await res.json();
      view.dispatch({
        effects: sqlCompartment.reconfigure(
          sql({ dialect: SQLite, schema: tables, upperCaseKeywords: true }),
        ),
      });
    } catch (err) {
      console.warn("SQL schema fetch failed:", err && err.message);
    }
  })();

  (async () => {
    try {
      const mod = await import(
        "https://esm.sh/codemirror-languageserver@1.22.0?bundle-deps&deps=@codemirror/state@6.5.2,@codemirror/view@6.38.1,@codemirror/autocomplete@6.18.6,@codemirror/language@6.10.3,@codemirror/lint@6.8.5"
      );
      const wsProto = window.location.protocol === "https:" ? "wss:" : "ws:";
      const lsp = mod.languageServer({
        serverUri: `${wsProto}//${window.location.host}/sql/lsp${query}`,
        rootUri: "file:///",
        documentUri: "file:///query.sql",
        languageId: "sql",
        workspaceFolders: null,
      });
      view.dispatch({ effects: lspCompartment.reconfigure(lsp) });
    } catch (err) {
      console.warn("SQL LSP unavailable:", err && err.message);
    }
  })();
}
