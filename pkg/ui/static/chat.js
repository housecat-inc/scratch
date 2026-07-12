(() => {
  const messages = document.getElementById("chat-messages");
  if (!messages) return;
  const scroll = () => window.scrollTo(0, document.body.scrollHeight);
  new MutationObserver(scroll).observe(messages, {
    characterData: true,
    childList: true,
    subtree: true,
  });
  scroll();
})();
