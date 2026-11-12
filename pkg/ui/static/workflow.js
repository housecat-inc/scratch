(() => {
  if (window.scratchWorkflow) return;

  const showAlert = (text) => {
    let alert = document.getElementById("chat-alert");
    if (!alert) {
      alert = document.createElement("div");
      alert.id = "chat-alert";
      document.body.appendChild(alert);
    }
    alert.textContent = text.trim() || "request failed";
    alert.classList.add("visible");
    clearTimeout(alert.hideTimer);
    alert.hideTimer = setTimeout(() => alert.classList.remove("visible"), 5000);
  };

  const initEditableElicit = (form) => {
    if (form.dataset.elicitEditableInitialized === "true") return;
    form.dataset.elicitEditableInitialized = "true";
    const fields = Array.from(form.querySelectorAll("input[name^='f_'], select[name^='f_'], textarea[name^='f_']"));
    const original = fields.map((field) => ({ checked: field.checked, field, value: field.value }));
    const lock = () => {
      form.classList.remove("editing");
      for (const { checked, field, value } of original) {
        if ("checked" in field) field.checked = checked;
        field.value = value;
        field.disabled = true;
      }
    };
    const unlock = () => {
      form.classList.add("editing");
      for (const field of fields) field.disabled = false;
      fields[0]?.focus();
    };
    form.addEventListener("click", (e) => {
      if (e.target.closest("[data-elicit-edit]") && !form.classList.contains("editing")) {
        e.preventDefault();
        unlock();
      } else if (e.target.closest("[data-elicit-cancel]")) {
        e.preventDefault();
        lock();
      }
    });
  };

  const initMessages = (messages) => {
    if (messages.dataset.chatMessagesInitialized === "true") return;
    messages.dataset.chatMessagesInitialized = "true";

    const pendingElicitation = (root) => root.querySelector("#elicit-form [name=elicitation_id]")?.value;
    messages.addEventListener("htmx:sse:before:message", (e) => {
      const current = pendingElicitation(document);
      if (!current) return;
      const incoming = new DOMParser().parseFromString(e.detail.message.data, "text/html");
      if (pendingElicitation(incoming) === current) e.preventDefault();
    });

    const toggled = new Map();
    messages.addEventListener("click", (e) => {
      const details = e.target.closest("details[data-key]");
      if (!details || !e.target.closest("summary")) return;
      toggled.set(details.dataset.key, !details.open);
    });
    const applyToggles = () => {
      for (const details of messages.querySelectorAll("details[data-key]")) {
        const open = toggled.get(details.dataset.key);
        if (open !== undefined && details.open !== open) details.open = open;
      }
    };

    const scroller = messages.closest(".mail-chat-body") || messages;
    const nearBottom = () => scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 80;
    let stick = true;
    scroller.addEventListener("scroll", () => {
      stick = nearBottom();
    });
    new MutationObserver(() => {
      applyToggles();
      if (stick) scroller.scrollTo(0, scroller.scrollHeight);
    }).observe(messages, { characterData: true, childList: true, subtree: true });
    scroller.scrollTo(0, scroller.scrollHeight);
  };

  const initAll = (root = document) => {
    if (root.matches?.(".chat-elicit-editable")) initEditableElicit(root);
    if (root.matches?.("[data-chat-messages]")) initMessages(root);
    root.querySelectorAll(".chat-elicit-editable").forEach(initEditableElicit);
    root.querySelectorAll("[data-chat-messages]").forEach(initMessages);
  };

  document.body.addEventListener("htmx:response:error", (e) => showAlert(e.detail.ctx.text));
  document.body.addEventListener("htmx:sse:error", (e) => e.preventDefault());
  new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      for (const node of mutation.addedNodes) {
        if (node.nodeType === 1) initAll(node);
      }
    }
  }).observe(document.body, { childList: true, subtree: true });

  window.scratchWorkflow = { init: initAll };
  initAll();
})();
