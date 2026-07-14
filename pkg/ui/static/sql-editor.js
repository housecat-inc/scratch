(function () {
  var CM = window.CMSQL;
  var mount = document.getElementById("sql-cm");
  var ta = document.getElementById("sql-editor");
  if (!CM || !mount || !ta) return;

  var Compartment = CM.Compartment;
  var EditorState = CM.EditorState;
  var EditorView = CM.EditorView;

  var sqlCompartment = new Compartment();
  var lspCompartment = new Compartment();

  var sync = EditorView.updateListener.of(function (update) {
    if (update.docChanged) ta.value = update.state.doc.toString();
  });

  var heightTheme = EditorView.theme({
    "&": { height: "180px", fontSize: "13px" },
    ".cm-scroller": { overflow: "auto" },
  });

  function runForm() {
    var form = document.getElementById("sql-run-form");
    if (form && window.htmx) window.htmx.trigger(form, "submit");
    return true;
  }

  var view = new EditorView({
    state: EditorState.create({
      doc: ta.value,
      extensions: [
        CM.lineNumbers(),
        CM.history(),
        CM.keymap.of([
          { key: "Mod-Enter", run: runForm },
          { key: "Tab", run: CM.acceptCompletion },
        ]),
        CM.keymap.of([].concat(CM.defaultKeymap, CM.historyKeymap, CM.completionKeymap)),
        sqlCompartment.of(CM.sql({ dialect: CM.SQLite })),
        CM.syntaxHighlighting(CM.defaultHighlightStyle),
        CM.autocompletion(),
        heightTheme,
        sync,
        lspCompartment.of([]),
      ],
    }),
    parent: mount,
  });

  ta.style.display = "none";
  ta.value = view.state.doc.toString();

  window.sqlEditor = view;
  window.sqlGetEditor = function () { return view.state.doc.toString(); };
  window.sqlSetEditor = function (text) {
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text || "" } });
  };

  var path = (window.sqlPath && window.sqlPath()) || "";
  var query = path ? "?path=" + encodeURIComponent(path) : "";

  fetch("/sql/schema" + query, { credentials: "same-origin" })
    .then(function (r) { return r.ok ? r.json() : null; })
    .then(function (data) {
      if (!data) return;
      view.dispatch({
        effects: sqlCompartment.reconfigure(
          CM.sql({ dialect: CM.SQLite, schema: data.tables, upperCaseKeywords: true }),
        ),
      });
    })
    .catch(function (err) { console.warn("SQL schema fetch failed:", err && err.message); });

  var formatter = null;
  function loadFormatter() {
    if (formatter) return Promise.resolve(formatter);
    return fetch("/static/lax-sql.wasm", { credentials: "same-origin" })
      .then(function (r) { if (!r.ok) throw new Error("wasm " + r.status); return r.arrayBuffer(); })
      .then(function (buf) {
        formatter = CM.createFromBuffer(buf);
        try { formatter.setConfig({}, {}); } catch (e) {}
        return formatter;
      });
  }

  var fmtBtn = document.getElementById("sql-fmt-btn");
  if (fmtBtn) {
    fmtBtn.addEventListener("click", function () {
      loadFormatter()
        .then(function (f) {
          var text = view.state.doc.toString();
          var out = f.formatText({ filePath: "query.sql", fileText: text });
          if (typeof out === "string" && out && out !== text) window.sqlSetEditor(out);
        })
        .catch(function (err) { console.warn("SQL format failed:", err && err.message); });
    });
  }

  try {
    var wsProto = window.location.protocol === "https:" ? "wss:" : "ws:";
    var lsp = CM.languageServer({
      serverUri: wsProto + "//" + window.location.host + "/sql/lsp" + query,
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
