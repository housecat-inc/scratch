(function () {
  function bind(wrap, id, label) {
    var value = wrap.querySelector("[data-contact-value]");
    if (value) value.value = id;
    var search = wrap.querySelector("[data-contact-search]");
    if (search) search.value = "";
    var chipLabel = wrap.querySelector("[data-contact-label]");
    if (chipLabel) chipLabel.textContent = label;
    var results = wrap.querySelector("[data-contact-results]");
    if (results) results.innerHTML = "";
    wrap.classList.add("contact-field-selected");
  }

  function clear(wrap) {
    var value = wrap.querySelector("[data-contact-value]");
    if (value) value.value = "";
    var search = wrap.querySelector("[data-contact-search]");
    if (search) search.value = "";
    var results = wrap.querySelector("[data-contact-results]");
    if (results) results.innerHTML = "";
    wrap.classList.remove("contact-field-selected");
    if (search) search.focus();
  }

  document.addEventListener("click", function (event) {
    var clearButton = event.target.closest("[data-contact-clear]");
    if (clearButton) {
      event.preventDefault();
      var clearWrap = clearButton.closest("[data-contact-field]");
      if (clearWrap) clear(clearWrap);
      return;
    }

    var option = event.target.closest("[data-contact-option]");
    if (option) {
      event.preventDefault();
      var wrap = option.closest("[data-contact-field]");
      if (!wrap) return;
      if (option.hasAttribute("data-contact-create")) {
        var name = option.getAttribute("data-label");
        fetch("/contacts", {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          body: "name=" + encodeURIComponent(name),
        })
          .then(function (r) { return r.json(); })
          .then(function (c) { bind(wrap, c.id, c.name); })
          .catch(function () {});
        return;
      }
      bind(wrap, option.getAttribute("data-id"), option.getAttribute("data-label"));
      return;
    }
    if (event.target.closest("[data-contact-field]")) return;
    document.querySelectorAll("[data-contact-results]").forEach(function (r) {
      r.innerHTML = "";
    });
  });
})();
