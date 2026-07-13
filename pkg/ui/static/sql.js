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

  document.addEventListener("click", function (e) {
    var el = e.target.closest(".sql-load");
    if (!el) return;
    e.preventDefault();
    loadSQL(el.getAttribute("data-sql") || "", true);
  });

  document.addEventListener("htmx:configRequest", function () {
    if (editor) editor.save();
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
