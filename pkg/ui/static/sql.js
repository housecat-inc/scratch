(function () {
  var ta = document.getElementById("sql-editor");

  window.sqlGetEditor = function () { return ta ? ta.value : ""; };
  window.sqlSetEditor = function (text) { if (ta) ta.value = text || ""; };

  window.sqlPath = function () {
    var el = document.querySelector('#sql-run-form input[name="path"]');
    return el ? el.value : "";
  };

  function runQuery() {
    var form = document.getElementById("sql-run-form");
    if (form) htmx.trigger(form, "submit");
  }

  function loadSQL(sql, run) {
    window.sqlSetEditor(sql);
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
    if (ta && window.sqlGetEditor) ta.value = window.sqlGetEditor();
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
})();
