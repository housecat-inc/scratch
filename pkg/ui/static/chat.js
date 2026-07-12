(() => {
  const messages = document.getElementById("chat-messages");
  if (!messages) return;

  messages.addEventListener("htmx:sseBeforeMessage", (e) => {
    const form = document.getElementById("elicit-form");
    if (!form) return;
    const id = form.querySelector("[name=elicitation_id]");
    if (id && e.detail.data.includes(`value="${id.value}"`)) {
      e.preventDefault();
    }
  });

  const scroll = () => window.scrollTo(0, document.body.scrollHeight);
  new MutationObserver(scroll).observe(messages, {
    characterData: true,
    childList: true,
    subtree: true,
  });
  scroll();
})();
