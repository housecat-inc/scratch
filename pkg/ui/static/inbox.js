(() => {
  const form = document.querySelector(".mail-universal-compose");
  if (!form || form.id === "chat-form") return;

  const agent = form.querySelector("[name=agent]");
  const input = form.querySelector("[name=prompt]");
  const radios = Array.from(form.querySelectorAll("[name=mode]"));

  const sync = () => {
    const mode = form.querySelector("[name=mode]:checked")?.value || "auto";
    if (input) {
      input.placeholder = form.dataset[`placeholder${mode[0].toUpperCase()}${mode.slice(1)}`] || form.dataset.placeholderAuto || input.placeholder;
    }
    if (agent) {
      agent.disabled = mode === "task" || mode === "workflow";
      agent.classList.toggle("disabled", agent.disabled);
    }
  };

  radios.forEach((radio) => radio.addEventListener("change", sync));
  sync();
})();
