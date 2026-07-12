(() => {
  const modelOptions = {
    claude: [
      ["default", "Default"],
      ["opus", "Opus"],
      ["sonnet", "Sonnet"],
      ["haiku", "Haiku"],
    ],
    codex: [
      ["default", "Default"],
      ["gpt-5.5", "GPT 5.5"],
      ["gpt-5.1", "GPT 5.1"],
      ["gpt-5", "GPT 5"],
    ],
    echo: [["default", "Default"]],
  };

  const agentKey = "scratch.chat.agent";
  const modelKey = (agent) => `scratch.chat.model.${agent || "default"}`;

  const hasOption = (select, value) => Array.from(select.options).some((option) => option.value === value);

  const fillModels = (select, agent, selected) => {
    const options = modelOptions[agent] || modelOptions.echo;
    select.replaceChildren(
      ...options.map(([value, label]) => {
        const option = document.createElement("option");
        option.value = value;
        option.textContent = label;
        return option;
      }),
    );
    select.value = options.some(([value]) => value === selected) ? selected : options[0][0];
  };

  document.querySelectorAll("[data-agent-model-form]").forEach((form) => {
    const agent = form.querySelector("[name=agent]");
    const model = form.querySelector("[name=model]");
    if (!agent || !model) return;

    const storedAgent = localStorage.getItem(agentKey);
    if (storedAgent && hasOption(agent, storedAgent)) {
      agent.value = storedAgent;
    }

    const syncModel = () => {
      const selectedAgent = agent.value;
      fillModels(model, selectedAgent, localStorage.getItem(modelKey(selectedAgent)) || model.value);
      localStorage.setItem(agentKey, selectedAgent);
      localStorage.setItem(modelKey(selectedAgent), model.value);
    };

    agent.addEventListener("change", syncModel);
    model.addEventListener("change", () => {
      localStorage.setItem(modelKey(agent.value), model.value);
    });
    syncModel();
  });

  document.addEventListener("click", (event) => {
    const trigger = event.target.closest("[data-trash-trigger]");
    if (trigger) {
      event.preventDefault();
      event.stopPropagation();
      const form = trigger.closest("[data-trash-form]");
      if (!form) return;
      document.querySelectorAll("[data-trash-form].confirming").forEach((other) => {
        if (other !== form) other.classList.remove("confirming");
      });
      form.classList.add("confirming");
      form.querySelector("[data-trash-submit]")?.focus();
      return;
    }

    const cancel = event.target.closest("[data-trash-cancel]");
    if (cancel) {
      event.preventDefault();
      event.stopPropagation();
      cancel.closest("[data-trash-form]")?.classList.remove("confirming");
    }
  });
})();
