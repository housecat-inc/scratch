(() => {
  const draftKeyPrefix = "scratch.chat.draft:";
  const popoutKey = "scratch.chat.popout";

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

  const imageIconHTML = () =>
    `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"></rect><circle cx="9" cy="9" r="2"></circle><path d="M21 15l-4.6-4.6a2 2 0 00-2.8 0L5 19"></path></svg>`;

  const paperclipIconHTML = () =>
    `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21.44 11.05l-9.19 9.19a6 6 0 01-8.49-8.49l9.19-9.19a4 4 0 015.66 5.66l-9.2 9.19a2 2 0 01-2.83-2.83l8.49-8.48"></path></svg>`;

  const state = () => {
    try {
      return JSON.parse(sessionStorage.getItem(popoutKey) || "{}");
    } catch {
      return {};
    }
  };

  const saveState = (next) => {
    const merged = { ...state(), ...next };
    if (!merged.threadID) {
      sessionStorage.removeItem(popoutKey);
      return;
    }
    sessionStorage.setItem(popoutKey, JSON.stringify(merged));
  };

  const clearState = () => sessionStorage.removeItem(popoutKey);

  const panelMode = (panel) => {
    if (panel.classList.contains("fullscreen")) return "fullscreen";
    if (panel.classList.contains("minimized")) return "minimized";
    return "open";
  };

  const applyPanelMode = (panel, mode) => {
    panel.classList.toggle("fullscreen", mode === "fullscreen");
    panel.classList.toggle("minimized", mode === "minimized");
    saveState({ mode, threadID: panel.dataset.threadId });
  };

  const insertPopout = (html, mode = "open") => {
    document.getElementById("floating-chat")?.remove();
    document.body.insertAdjacentHTML("beforeend", html);
    const panel = document.getElementById("floating-chat");
    if (!panel) return null;
    applyPanelMode(panel, mode);
    window.htmx?.process(panel);
    initAll(panel);
    panel.querySelector("[data-chat-input]")?.focus();
    return panel;
  };

  const openPopout = async (url, mode = "open") => {
    const res = await fetch(url);
    if (!res.ok) {
      showAlert(await res.text());
      return null;
    }
    return insertPopout(await res.text(), mode);
  };

  const restorePopout = () => {
    const saved = state();
    if (!saved.threadID) return;
    openPopout(`/chat/${saved.threadID}/popout`, saved.mode || "open");
  };

  const showInboxBehindPopout = (threadID) => {
    if (location.pathname === `/inbox/chats/${threadID}`) {
      location.assign("/inbox/chats");
    }
  };

  const storageGet = (key) => {
    try {
      return sessionStorage.getItem(key) || "";
    } catch {
      return "";
    }
  };

  const storageRemove = (key) => {
    try {
      sessionStorage.removeItem(key);
    } catch {}
  };

  const storageSet = (key, value) => {
    try {
      if (value) sessionStorage.setItem(key, value);
      else sessionStorage.removeItem(key);
    } catch {}
  };

  const composerDraftKey = (form) => {
    const action = form.getAttribute("action") || form.getAttribute("hx-post") || location.pathname;
    const agent = form.querySelector('[name="agent"]')?.value || "";
    const mode = form.querySelector('[name="mode"]')?.value || "";
    const model = form.querySelector('[name="model"]')?.value || "";
    const popoutThreadID = form.closest("[data-chat-popout]")?.dataset.threadId || "";
    const threadMatch = action.match(/^\/chat\/(\d+)\/messages$/);
    if (popoutThreadID || threadMatch) return draftKeyPrefix + "thread:" + (popoutThreadID || threadMatch[1]);
    return draftKeyPrefix + ["compose", action, mode, agent, model].join(":");
  };

  const initComposer = (form) => {
    if (form.dataset.chatInitialized === "true") return;
    form.dataset.chatInitialized = "true";

    const strip = form.querySelector("[data-chat-attachments]");
    const input = form.querySelector("[data-chat-input]");
    if (!strip || !input) return;

    const draftKey = composerDraftKey(form);
    const uploadURL = form.dataset.uploadUrl;
    const fileInput = form.querySelector("[data-chat-file]");
    const draftFiles = [];
    const previewURLKey = Symbol("previewURL");

    const draftPreviewURL = (file) => {
      if (!file[previewURLKey]) file[previewURLKey] = URL.createObjectURL(file);
      return file[previewURLKey];
    };

    const revokeDraftPreview = (file) => {
      if (!file?.[previewURLKey]) return;
      URL.revokeObjectURL(file[previewURLKey]);
      delete file[previewURLKey];
    };

    const revokeDraftPreviews = () => {
      for (const file of draftFiles) revokeDraftPreview(file);
    };

    const syncDraftInput = () => {
      if (!fileInput || uploadURL || !window.DataTransfer) return;
      const transfer = new DataTransfer();
      for (const file of draftFiles) transfer.items.add(file);
      fileInput.files = transfer.files;
    };

    const draftChip = (file, index) => {
      const chip = document.createElement("span");
      chip.className = "chat-attachment-chip";
      chip.dataset.draftIndex = String(index);
      if (file.type.startsWith("image/")) {
        chip.insertAdjacentHTML("beforeend", imageIconHTML());
        const img = document.createElement("img");
        img.className = "chat-chip-preview";
        img.alt = file.name;
        img.loading = "lazy";
        img.src = draftPreviewURL(file);
        chip.appendChild(img);
      } else {
        chip.insertAdjacentHTML("beforeend", paperclipIconHTML());
      }
      const name = document.createElement("span");
      name.className = "chat-chip-name";
      name.textContent = file.name;
      chip.appendChild(name);
      const remove = document.createElement("button");
      remove.type = "button";
      remove.className = "chat-attachment-remove";
      remove.dataset.draftIndex = String(index);
      remove.ariaLabel = "Remove " + file.name;
      remove.textContent = "×";
      chip.appendChild(remove);
      return chip;
    };

    const renderDraftFiles = () => {
      if (uploadURL) return;
      strip.replaceChildren(...draftFiles.map((file, index) => draftChip(file, index)));
      syncDraftInput();
    };

    const addDraftFiles = (files) => {
      draftFiles.push(...files);
      renderDraftFiles();
    };

    const setComposeMode = (mode) => {
      const modeInput = form.querySelector('input[name="mode"]');
      if (modeInput) modeInput.value = mode;
    };

    const clearTextDraft = () => storageRemove(draftKey);

    const resize = () => {
      input.style.height = "auto";
      input.style.height = Math.min(input.scrollHeight, 200) + "px";
    };

    const restoreTextDraft = () => {
      const draft = storageGet(draftKey);
      if (draft && !input.value) input.value = draft;
      resize();
    };

    const saveTextDraft = () => storageSet(draftKey, input.value);

    const upload = async (file) => {
      if (!uploadURL) {
        addDraftFiles([file]);
        return;
      }
      const body = new FormData();
      body.append("file", file, file.name || "pasted-image.png");
      const res = await fetch(uploadURL, { body, method: "POST" });
      if (!res.ok) {
        showAlert(await res.text());
        return;
      }
      strip.insertAdjacentHTML("beforeend", await res.text());
    };

    const uploadAll = async (files) => {
      for (const file of files) await upload(file);
    };

    input.addEventListener("input", () => {
      resize();
      saveTextDraft();
    });
    input.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
        e.preventDefault();
        form.requestSubmit();
      }
    });
    restoreTextDraft();

    fileInput?.addEventListener("change", (e) => {
      uploadAll(e.target.files);
      if (uploadURL) e.target.value = "";
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
      if (!uploadURL) {
        const index = Number(remove.dataset.draftIndex);
        if (Number.isInteger(index)) {
          const removed = draftFiles.splice(index, 1);
          for (const file of removed) revokeDraftPreview(file);
          renderDraftFiles();
        }
        return;
      }
      await fetch(uploadURL + "/" + remove.dataset.id, { method: "DELETE" });
      remove.closest(".chat-attachment-chip")?.remove();
    });
    form.addEventListener("htmx:afterRequest", (e) => {
      if (e.detail.successful && e.detail.xhr?.status === 204) {
        clearTextDraft();
        revokeDraftPreviews();
        strip.replaceChildren();
        resize();
        form.closest("[data-chat-popout]")?.setAttribute("data-empty", "false");
      }
    });
    form.addEventListener("submit", () => {
      if (!uploadURL) {
        clearTextDraft();
        revokeDraftPreviews();
      }
    });

    initCapture(form, input, upload, addDraftFiles, setComposeMode);
  };

  const initCapture = (form, input, upload, addDraftFiles, setComposeMode) => {
    const snap = form.querySelector("[data-chat-snap]");
    if (!snap) return;

    const overlay = document.createElement("div");
    overlay.id = "chat-select-overlay";
    const label = document.createElement("span");
    label.className = "chat-select-label";
    overlay.appendChild(label);
    let target = null;

    const sectionOf = (el) => {
      const preferred = el.closest(
        ".chat-turn, .chat-row, .chat-plan, .chat-md, .chat-bubble, .mail-reader-head, .mail-list-item, .gm-label, article, section, aside, nav, form, header, footer"
      );
      if (preferred) return preferred;
      let node = el;
      while (
        node.parentElement &&
        node.parentElement !== document.body &&
        (node.getBoundingClientRect().width < 80 || node.getBoundingClientRect().height < 32)
      ) {
        node = node.parentElement;
      }
      return node;
    };

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
      let el = document.elementFromPoint(e.clientX, e.clientY);
      if (el && (el.closest("[data-chat-popout]") || el.closest("[data-chat-form]") || el.closest(".mail-footer"))) {
        el = null;
      }
      if (!el || el === document.body || el === document.documentElement) {
        target = null;
        return;
      }
      target = sectionOf(el);
      const rect = target.getBoundingClientRect();
      overlay.style.display = "block";
      overlay.style.height = rect.height + "px";
      overlay.style.left = rect.left + "px";
      overlay.style.top = rect.top + "px";
      overlay.style.width = rect.width + "px";
      overlay.classList.toggle("label-inside", rect.top < 30);
      label.textContent = cssPath(target) + "  " + Math.round(rect.width) + "×" + Math.round(rect.height);
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
      if (document.body.classList.contains("chat-selecting")) return;
      document.body.classList.add("chat-selecting");
      document.body.appendChild(overlay);
      overlay.style.display = "none";
      document.addEventListener("mousemove", onMove, true);
      setTimeout(() => {
        document.addEventListener("click", onPick, true);
        document.addEventListener("keydown", onKey, true);
      }, 0);
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
      setComposeMode("chat");
      const selector = cssPath(el);
      const html = "<!-- url: " + location.href + " -->\n<!-- selector: " + selector + " -->\n" + el.outerHTML;
      const uploadURL = form.dataset.uploadUrl;
      if (uploadURL) {
        try {
          const canvas = await html2canvas(el, { logging: false, useCORS: true });
          const blob = await new Promise((resolve) => canvas.toBlob(resolve, "image/png"));
          if (blob) await upload(new File([blob], "selection.png", { type: "image/png" }));
        } catch (err) {
          showAlert("screenshot failed: " + err.message);
        }
        await upload(new File([html], "selection.html", { type: "text/html" }));
      } else {
        const files = [];
        try {
          const canvas = await html2canvas(el, { logging: false, useCORS: true });
          const blob = await new Promise((resolve) => canvas.toBlob(resolve, "image/png"));
          if (blob) files.push(new File([blob], "selection.png", { type: "image/png" }));
        } catch (err) {
          showAlert("screenshot failed: " + err.message);
        }
        files.push(new File([html], "selection.html", { type: "text/html" }));
        addDraftFiles(files);
      }
      const note = "Selected " + selector + " on " + location.href;
      input.value = input.value ? input.value + "\n" + note : note;
      input.dispatchEvent(new Event("input"));
      input.focus();
    };
  };

  const initMessages = (messages) => {
    if (messages.dataset.chatMessagesInitialized === "true") return;
    messages.dataset.chatMessagesInitialized = "true";

    const pendingElicitation = (root) => root.querySelector("#elicit-form [name=elicitation_id]")?.value;

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
        if (open !== undefined && details.open !== open) details.open = open;
      }
    };

    const scroller = messages.closest("[data-chat-scroller]") || messages.closest(".mail-chat-body") || messages;
    const nearBottom = () => scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 80;
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
  };

  const initPopout = (panel) => {
    if (panel.dataset.chatPanelInitialized === "true") return;
    panel.dataset.chatPanelInitialized = "true";

    panel.addEventListener("click", async (e) => {
      const minimize = e.target.closest("[data-chat-minimize]");
      if (minimize) {
        applyPanelMode(panel, panel.classList.contains("minimized") ? "open" : "minimized");
        return;
      }
      const restore = e.target.closest("[data-chat-restore]");
      if (restore) {
        applyPanelMode(panel, "open");
        return;
      }
      const fullscreen = e.target.closest("[data-chat-fullscreen]");
      if (fullscreen) {
        applyPanelMode(panel, panel.classList.contains("fullscreen") ? "open" : "fullscreen");
        return;
      }
      const close = e.target.closest("[data-chat-close]");
      if (close) {
        const empty = panel.dataset.empty === "true";
        const threadID = panel.dataset.threadId;
        panel.remove();
        clearState();
        if (empty && threadID) await fetch(`/chat/${threadID}`, { method: "DELETE" });
        return;
      }
      if (panel.classList.contains("minimized") && e.target.closest(".floating-chat-head")) {
        applyPanelMode(panel, "open");
      }
    });
  };

  const initAll = (root = document) => {
    if (root.matches?.("[data-chat-form]")) initComposer(root);
    if (root.matches?.("[data-chat-messages]")) initMessages(root);
    if (root.matches?.("[data-chat-popout]")) initPopout(root);
    root.querySelectorAll("[data-chat-form]").forEach(initComposer);
    root.querySelectorAll("[data-chat-messages]").forEach(initMessages);
    root.querySelectorAll("[data-chat-popout]").forEach(initPopout);
  };

  document.body.addEventListener("htmx:responseError", (e) => {
    showAlert(e.detail.xhr.responseText);
  });

  document.addEventListener("submit", (e) => {
    const form = e.target.closest("[data-chat-popout-new]");
    if (!form) return;
    e.preventDefault();
    const params = new URLSearchParams(new FormData(form));
    openPopout(`/chat/popout/new?${params.toString()}`);
  });

  document.addEventListener("click", async (e) => {
    const trigger = e.target.closest("[data-chat-popout-thread]");
    if (!trigger) return;
    e.preventDefault();
    const threadID = trigger.dataset.chatPopoutThread;
    const panel = await openPopout(`/chat/${threadID}/popout`);
    if (panel) showInboxBehindPopout(threadID);
  });

  new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      for (const node of mutation.addedNodes) {
        if (node.nodeType === 1) initAll(node);
      }
    }
  }).observe(document.body, { childList: true, subtree: true });

  window.scratchChat = { init: initAll, openPopout };
  initAll();
  restorePopout();
})();
