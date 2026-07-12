(() => {
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

  document.body.addEventListener("htmx:responseError", (e) => {
    showAlert(e.detail.xhr.responseText);
  });

  const form = document.getElementById("chat-form");
  const strip = document.getElementById("chat-attachments");
  if (form && strip) {
    const uploadURL = form.dataset.uploadUrl;
    const upload = async (file) => {
      const body = new FormData();
      body.append("file", file, file.name || "pasted-image.png");
      const res = await fetch(uploadURL, { body, method: "POST" });
      if (!res.ok) {
        showAlert(await res.text());
        return;
      }
      strip.insertAdjacentHTML("beforeend", await res.text());
    };
    const uploadAll = (files) => {
      for (const file of files) upload(file);
    };

    document.getElementById("chat-file")?.addEventListener("change", (e) => {
      uploadAll(e.target.files);
      e.target.value = "";
    });
    document.getElementById("chat-input")?.addEventListener("paste", (e) => {
      if (e.clipboardData?.files?.length) {
        e.preventDefault();
        uploadAll(e.clipboardData.files);
      }
    });
    form.addEventListener("dragover", (e) => {
      e.preventDefault();
      form.classList.add("drop-target");
    });
    form.addEventListener("dragleave", () => form.classList.remove("drop-target"));
    form.addEventListener("drop", (e) => {
      e.preventDefault();
      form.classList.remove("drop-target");
      if (e.dataTransfer?.files?.length) uploadAll(e.dataTransfer.files);
    });
    strip.addEventListener("click", async (e) => {
      const remove = e.target.closest(".chat-attachment-remove");
      if (!remove) return;
      await fetch(uploadURL + "/" + remove.dataset.id, { method: "DELETE" });
      remove.closest(".chat-attachment-chip")?.remove();
    });
    form.addEventListener("htmx:afterRequest", (e) => {
      if (e.detail.successful && e.detail.xhr?.status === 204) {
        strip.replaceChildren();
      }
    });
  }

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
