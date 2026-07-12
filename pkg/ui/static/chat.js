(() => {
  const messages = document.getElementById("chat-messages");
  if (!messages) return;

  const pendingElicitation = (root) =>
    root.querySelector("#elicit-form [name=elicitation_id]")?.value;

  messages.addEventListener("htmx:sseBeforeMessage", (e) => {
    const current = pendingElicitation(document);
    if (!current) return;
    const incoming = new DOMParser().parseFromString(e.detail.data, "text/html");
    if (pendingElicitation(incoming) === current) {
      e.preventDefault();
    }
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
      if (open !== undefined && details.open !== open) {
        details.open = open;
      }
    }
  };

  document.body.addEventListener("htmx:responseError", (e) => {
    let alert = document.getElementById("chat-alert");
    if (!alert) {
      alert = document.createElement("div");
      alert.id = "chat-alert";
      document.body.appendChild(alert);
    }
    alert.textContent = e.detail.xhr.responseText.trim() || "request failed";
    alert.classList.add("visible");
    clearTimeout(alert.hideTimer);
    alert.hideTimer = setTimeout(() => alert.classList.remove("visible"), 5000);
  });

  const scroller = messages.closest(".mail-chat-body") || messages;
  const nearBottom = () =>
    scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 80;
  let stick = true;
  scroller.addEventListener("scroll", () => {
    stick = nearBottom();
  });
  new MutationObserver(() => {
    applyToggles();
    if (stick) scroller.scrollTo(0, scroller.scrollHeight);
  }).observe(messages, {
    characterData: true,
    childList: true,
    subtree: true,
  });
  scroller.scrollTo(0, scroller.scrollHeight);
})();
