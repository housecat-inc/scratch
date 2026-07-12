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

  const scroll = () => window.scrollTo(0, document.body.scrollHeight);
  new MutationObserver(scroll).observe(messages, {
    characterData: true,
    childList: true,
    subtree: true,
  });
  scroll();
})();
