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
  const input = document.getElementById("chat-input");
  if (form && strip && input) {
    const uploadURL = form.dataset.uploadUrl;

    const resize = () => {
      input.style.height = "auto";
      input.style.height = Math.min(input.scrollHeight, 200) + "px";
    };
    input.addEventListener("input", resize);
    input.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
        e.preventDefault();
        form.requestSubmit();
      }
    });
    resize();

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
    input.addEventListener("paste", (e) => {
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
        resize();
      }
    });

    const snap = document.getElementById("chat-snap");
    if (snap) {
      const overlay = document.createElement("div");
      overlay.id = "chat-select-overlay";
      let target = null;

      const stopSelecting = () => {
        target = null;
        overlay.remove();
        document.body.classList.remove("chat-selecting");
        document.removeEventListener("mousemove", onMove, true);
        document.removeEventListener("click", onPick, true);
        document.removeEventListener("keydown", onKey, true);
      };
      const onMove = (e) => {
        overlay.style.display = "none";
        target = document.elementFromPoint(e.clientX, e.clientY);
        overlay.style.display = "";
        if (!target || target === document.body || target === document.documentElement) {
          target = null;
          overlay.style.display = "none";
          return;
        }
        const rect = target.getBoundingClientRect();
        overlay.style.height = rect.height + "px";
        overlay.style.left = rect.left + "px";
        overlay.style.top = rect.top + "px";
        overlay.style.width = rect.width + "px";
      };
      const onKey = (e) => {
        if (e.key === "Escape") stopSelecting();
      };
      const onPick = (e) => {
        e.preventDefault();
        e.stopPropagation();
        const el = target;
        stopSelecting();
        if (el) capture(el);
      };
      snap.addEventListener("click", () => {
        document.body.classList.add("chat-selecting");
        document.body.appendChild(overlay);
        document.addEventListener("mousemove", onMove, true);
        document.addEventListener("click", onPick, true);
        document.addEventListener("keydown", onKey, true);
      });

      const cssPath = (el) => {
        const parts = [];
        for (let node = el; node && node.nodeType === 1 && parts.length < 6; node = node.parentElement) {
          if (node.id) {
            parts.unshift("#" + node.id);
            break;
          }
          let part = node.tagName.toLowerCase();
          const cls = [...node.classList].slice(0, 2).join(".");
          if (cls) part += "." + cls;
          const siblings = node.parentElement
            ? [...node.parentElement.children].filter((c) => c.tagName === node.tagName)
            : [];
          if (siblings.length > 1) {
            part += `:nth-of-type(${siblings.indexOf(node) + 1})`;
          }
          parts.unshift(part);
        }
        return parts.join(" > ");
      };

      const capture = async (el) => {
        const selector = cssPath(el);
        let html = el.outerHTML;
        if (html.length > 1500) html = html.slice(0, 1500) + "…";
        try {
          const canvas = await html2canvas(el, { logging: false, useCORS: true });
          const blob = await new Promise((resolve) => canvas.toBlob(resolve, "image/png"));
          if (blob) {
            await upload(new File([blob], "selection.png", { type: "image/png" }));
          }
        } catch (err) {
          showAlert("screenshot failed: " + err.message);
        }
        const note = "Selected element: " + selector + "\n```html\n" + html + "\n```\n";
        input.value = input.value ? input.value + "\n" + note : note;
        input.dispatchEvent(new Event("input"));
        input.focus();
      };
    }
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
