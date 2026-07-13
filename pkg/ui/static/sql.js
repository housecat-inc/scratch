(function () {
  var editor = null;
  var schema = {};

  function pathValue() {
    var el = document.querySelector('#sql-run-form input[name="path"]');
    return el ? el.value : "";
  }

  function loadSchema() {
    fetch("/sql/schema?path=" + encodeURIComponent(pathValue()))
      .then(function (r) { return r.ok ? r.json() : { tables: {} }; })
      .then(function (data) {
        schema = data.tables || {};
        if (editor) editor.setOption("hintOptions", { tables: schema });
      })
      .catch(function () {});
  }

  function initEditor() {
    var ta = document.getElementById("sql-editor");
    if (!ta || !window.CodeMirror) return;
    editor = CodeMirror.fromTextArea(ta, {
      mode: "text/x-sqlite",
      theme: "neat",
      lineNumbers: true,
      lineWrapping: true,
      extraKeys: {
        "Cmd-Enter": runQuery,
        "Ctrl-Enter": runQuery,
        "Ctrl-Space": "autocomplete",
      },
    });
    editor.on("inputRead", function (cm, change) {
      if (change.text[0] && /[\w.]/.test(change.text[0])) {
        cm.showHint({ completeSingle: false, hint: CodeMirror.hint.sql, tables: schema });
      }
    });
  }

  function runQuery() {
    var form = document.getElementById("sql-run-form");
    if (form) htmx.trigger(form, "submit");
  }

  function loadSQL(sql, run) {
    if (editor) {
      editor.setValue(sql);
      editor.focus();
    } else {
      var ta = document.getElementById("sql-editor");
      if (ta) ta.value = sql;
    }
    var wrap = document.getElementById("sql-wrap");
    if (wrap) wrap.classList.add("show-editor");
    if (run) runQuery();
  }

  function openSaveModal() {
    var modal = document.getElementById("sql-save-modal");
    if (!modal) return;
    modal.classList.remove("hidden");
    var name = document.getElementById("sql-save-name");
    if (name) { name.focus(); name.select(); }
  }

  function closeSaveModal() {
    var modal = document.getElementById("sql-save-modal");
    if (modal) modal.classList.add("hidden");
  }

  document.addEventListener("click", function (e) {
    if (e.target.closest("[data-sql-save-open]")) {
      e.preventDefault();
      openSaveModal();
      return;
    }
    if (e.target.closest("[data-sql-save-close]")) {
      e.preventDefault();
      closeSaveModal();
      return;
    }
    var loader = e.target.closest("[data-sql]");
    if (loader) {
      e.preventDefault();
      loadSQL(loader.getAttribute("data-sql") || "", true);
    }
  });

  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") closeSaveModal();
  });

  document.addEventListener("htmx:configRequest", function () {
    if (editor) editor.save();
  });

  document.addEventListener("htmx:afterRequest", function (e) {
    if (e.target && e.target.id === "sql-save-confirm" && e.detail.successful) {
      closeSaveModal();
    }
  });

  var back = document.getElementById("sql-back-btn");
  if (back) {
    back.addEventListener("click", function () {
      var wrap = document.getElementById("sql-wrap");
      if (wrap) wrap.classList.remove("show-editor");
    });
  }

  initEditor();
  loadSchema();
})();
