import * as Arr from "effect/Array";
import * as Option from "effect/Option";
import * as Wire from "../schemas/schemas.gen";
import { readJSON, readResponseJSON } from "./failures";
import { on } from "./keys";
import { registerComponent } from "./lifecycle";
import { isErrorMessage } from "./request_outcome";
import { readDecoded, type WireContext } from "./wire";

const SAVED: WireContext = { endpoint: "/evals/:id/labels", operation: "hand-labels.autosave" };
const RENDERED: WireContext = { endpoint: "/evals/:id", operation: "hand-labels.tags" };

registerComponent('[data-slot="hand-labels"]', (root, signal) => {
  const form = () => root.querySelector<HTMLFormElement>('[data-slot="hand-labels-form"]');
  const initial = form();
  if (!initial) return;
  const snapshot = () => {
    const data = new URLSearchParams();
    for (const [key, value] of new FormData(form()!)) {
      if (typeof value === "string") data.set(key, value);
    }
    return data;
  };
  let base = snapshot().get("base") || "0";
  let queue = Promise.resolve();
  let pending = 0;
  let dirty = false;
  let blocked = false;
  let conflict = false;
  let savedComplete = initial.dataset.complete === "true";
  let navigating = false;
  const autoAdvance = initial.dataset.autoAdvance === "true";
  const advanceURL = initial.dataset.advanceUrl!;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const action = initial.action;
  const url = new URL(location.href);
  url.searchParams.set("message", snapshot().get("message")!);
  history.replaceState(history.state, "", url);
  root.focus({ preventScroll: true });
  let activeStep = "tags";
  const steps = () =>
    Array.from(root.querySelectorAll<HTMLElement>("[data-hand-label-step]"))
      .filter(step => step.hidden === false && !(step instanceof HTMLFieldSetElement && step.disabled));
  const markStep = (step: HTMLElement, focus = false) => {
    activeStep = step.dataset.handLabelStep!;
    for (const other of steps()) other.toggleAttribute("data-focused", other === step);
    if (focus) {
      const target = step.querySelector<HTMLElement>("[data-hand-label-tag-input]") ?? step;
      target.focus({ preventScroll: true });
    }
  };
  const refreshStep = () => {
    const step = steps().find(step => step.dataset.handLabelStep === activeStep);
    if (step) markStep(step);
  };
  refreshStep();
  root.addEventListener("focusin", (event) => {
    const step = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-hand-label-step]") : null;
    if (step) markStep(step);
  }, { signal });

  const status = (text: string) => {
    if (signal.aborted) return;
    const el = root.querySelector("[data-hand-label-status]");
    if (el && el.textContent !== text) el.textContent = text;
    el?.classList.toggle("sr-only", !blocked);
  };
  const post = async (url: string, data: URLSearchParams) => {
    data.set("base", base);
    // Accepted writes finish in order; removing the view only stops DOM updates.
    const response = await fetch(url, { method: "POST", body: data, keepalive: true });
    if (!response.ok) {
      conflict = response.status === 409;
      // A proxy's failure can answer HTML rather than the app's JSON error,
      // which leaves the generic message.
      const error = await readResponseJSON(response);
      throw new Error(isErrorMessage(error) ? error.message : "Could not save. Check your connection and retry.");
    }
    return response;
  };
  const persist = async (data: URLSearchParams, state = "auto") => {
    data.set("autosave", "true");
    data.set("state", state);
    const response = await post(action, data);
    const saved = readDecoded(Wire.HandLabelSaveOut, SAVED, await response.json());
    // The next save builds on the saved label's id, so only an id confirms the save.
    if (!Number.isSafeInteger(saved.id) || saved.id <= 0) {
      throw new Error("Could not confirm the save. Please reload.");
    }
    base = String(saved.id);
    savedComplete = saved.state === "complete";
    if (!signal.aborted) {
      for (const input of root.querySelectorAll<HTMLInputElement>('input[name="base"]')) input.value = base;
      const undo = root.querySelector<HTMLFormElement>('[data-slot="hand-labels-undo"]');
      if (undo) undo.hidden = saved.state === "withdrawn";
      const applySender = root.querySelector<HTMLInputElement>("[data-hand-label-apply-sender]");
      if (applySender !== null && applySender.checked && savedComplete) {
        const applyStatus = root.querySelector<HTMLElement>("[data-hand-label-apply-sender-status]");
        const applyError = saved.sender_apply_error ?? "";
        const applied = saved.applied_to_sender ?? 0;
        if (applyStatus) {
          applyStatus.textContent = applyError || `Updated ${applied} other ${applied === 1 ? "email" : "emails"}.`;
          applyStatus.classList.toggle("text-destructive", applyError !== "");
        }
        applySender.checked = false;
      }
    }
  };
  const enqueue = (operation: () => Promise<void>) => {
    pending++;
    if (!blocked) status("Saving…");
    queue = queue.then(async () => {
      try {
        if (!blocked) await operation();
      } catch (error) {
        blocked = true;
        status("Not saved: " + (error instanceof Error ? error.message : "Please retry."));
        if (!signal.aborted) {
          const retry = root.querySelector<HTMLElement>("[data-hand-label-retry]");
          const reload = root.querySelector<HTMLElement>("[data-hand-label-reload]");
          if (retry) retry.hidden = conflict;
          if (reload) reload.hidden = !conflict;
        }
      } finally {
        pending--;
        if (!pending && !blocked && !dirty) {
          status("Saved");
          if (autoAdvance && savedComplete && !signal.aborted) void navigate(advanceURL);
        }
      }
    });
    return queue;
  };
  const save = (state = "auto") => {
    clearTimeout(timer);
    dirty = false;
    const data = snapshot();
    return enqueue(() => persist(data, state));
  };
  const flush = async () => {
    if (dirty) void save();
    await queue;
    return !blocked;
  };
  const navigate = async (href: string) => {
    if (navigating) return;
    navigating = true;
    root.inert = true;
    if (await flush()) location.assign(href);
    else {
      navigating = false;
      root.inert = false;
    }
  };

  const continueNext = () => {
    const next = root.querySelector<HTMLAnchorElement>("[data-hand-label-next]");
    if (!next) return false;
    next.click();
    return true;
  };

  root.addEventListener("change", (event) => {
    const target = event.target;
    if (!(target instanceof Element)) return;
    if (target.matches('input[name="sender_type"]')) {
      confirmSender();
      return;
    }
    if (target instanceof HTMLInputElement && target.matches("[data-hand-label-apply-sender]")) {
      if (target.checked && savedComplete) void save();
      return;
    }
    if (target.matches('input[name^="choice_"]')) void save();
  }, { signal });
  const confirmSender = () => {
    const field = form()?.querySelector<HTMLInputElement>('input[name="sender_type_reviewed"]');
    const choice = form()?.querySelector<HTMLInputElement>('input[name="sender_type"]:checked');
    if (!field || !choice) return;
    if (field.value === "true" && field.dataset.confirmedValue === choice.value) return;
    field.value = "true";
    field.dataset.confirmedValue = choice.value;
    void save();
  };
  root.addEventListener("input", (event) => {
    if (!(event.target instanceof Element) || !event.target.matches('textarea[name="comment"]')) return;
    dirty = true;
    if (!blocked) status("Saving…");
    clearTimeout(timer);
    timer = setTimeout(() => {
      void save();
    }, 400);
  }, { signal });
  root.addEventListener("focusout", (event) => {
    if (dirty && event.target instanceof Element && event.target.matches('textarea[name="comment"]')) void save();
  }, { signal });
  root.addEventListener("submit", (event) => {
    if (event.target === form()) {
      event.preventDefault();
      void save();
    } else if ((pending || dirty || blocked) && event.target instanceof HTMLFormElement) {
      event.preventDefault();
      const other = event.target;
      const submitter = event.submitter;
      void flush().then(ok => {
        if (ok && !signal.aborted) other.requestSubmit(submitter);
      });
    }
  }, { signal });
  root.addEventListener("click", (event) => {
    const target = event.target;
    if (!(target instanceof Element)) return;
    const step = target.closest<HTMLElement>("[data-hand-label-step]");
    if (step) markStep(step);
    const sender = target.closest("label")?.querySelector<HTMLInputElement>('input[name="sender_type"]');
    if (sender?.checked === true && target === sender) confirmSender();
    const selected = target.closest("label")?.querySelector<HTMLInputElement>('input[name^="choice_"]');
    if (selected?.checked === true && target !== selected) void save();
    const skip = target.closest<HTMLElement>("[data-hand-label-skip]");
    if (skip) {
      event.preventDefault();
      void save("skipped");
      void navigate(skip.dataset.next!);
      return;
    }
    if (target.closest("[data-hand-label-retry]")) {
      blocked = false;
      conflict = false;
      target.closest<HTMLElement>("[data-hand-label-retry]")!.hidden = true;
      void save();
      return;
    }
    const link = target.closest<HTMLAnchorElement>("a[href]");
    if (
      link && !link.hasAttribute("data-hand-label-reload") && !event.metaKey && !event.ctrlKey && !event.shiftKey
      && event.button === 0 && !link.target && (pending || dirty || blocked)
    ) {
      event.preventDefault();
      void navigate(link.href);
    }
  }, { signal });

  const chooseDirection = (direction: number) => {
    const step = steps().find(step => step.dataset.handLabelStep === activeStep);
    if (!step) return;
    if (activeStep === "navigation") {
      const link = step.querySelector<HTMLAnchorElement>(
        direction < 0 ? "[data-hand-label-previous]" : "[data-hand-label-next]",
      );
      link?.click();
      return;
    }
    const options = Array.from(step.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
    if (!options.length) return;
    const current = options.findIndex(option => option.checked);
    const next = current < 0
      ? (direction < 0 ? 0 : options.length - 1)
      : options.length === 2
      ? 1 - current
      : Math.max(0, Math.min(options.length - 1, current + direction));
    const option = options[next];
    option.checked = true;
    option.dispatchEvent(new Event("change", { bubbles: true }));
  };
  const moveStep = (direction: number) => {
    const all = steps();
    const current = all.findIndex(step => step.dataset.handLabelStep === activeStep);
    if (direction > 0 && activeStep === "sender") confirmSender();
    const next = all[(current + direction + all.length) % all.length] as HTMLElement | undefined;
    if (next) markStep(next, true);
  };
  const tagInput = form()!.querySelector<HTMLInputElement>("[data-hand-label-tag-input]")!;
  const tagValue = form()!.querySelector<HTMLInputElement>('input[name="tags"]')!;
  const tagOptions = root.querySelector<HTMLElement>("#hand-labels-tag-options")!;
  const tagChips = root.querySelector<HTMLElement>('[data-slot="hand-labels-applied-tags"]')!;
  const tagError = root.querySelector<HTMLElement>('[data-slot="hand-labels-tag-error"]')!;
  const knownTags = Array.from(
    root.querySelectorAll<HTMLOptionElement>("#hand-labels-tag-catalog option"),
    option => option.value,
  );
  const drawnTags = readJSON(tagValue.value);
  let tags: string[] = drawnTags === undefined ? [] : [...(readDecoded(Wire.HandLabelTags, RENDERED, drawnTags) ?? [])];
  let tagMatches: string[] = [];
  let selectedTag = -1;
  const tagKey = (value: string) => value.toLowerCase();
  const closeTagOptions = () => {
    tagOptions.hidden = true;
    tagInput.setAttribute("aria-expanded", "false");
    tagInput.removeAttribute("aria-activedescendant");
    selectedTag = -1;
  };
  const filterTags = () => {
    const query = tagKey(tagInput.value.trim());
    tagMatches = Arr.filter(
      knownTags,
      tag => query !== "" && tagKey(tag).includes(query) && !Arr.some(tags, applied => tagKey(applied) === tagKey(tag)),
    );
    selectedTag = tagMatches.length ? 0 : -1;
    tagOptions.replaceChildren();
    const template = root.querySelector<HTMLTemplateElement>('[data-slot="hand-labels-tag-option-template"]')!;
    for (const [index, tag] of tagMatches.entries()) {
      const option = template.content.firstElementChild?.cloneNode(true);
      if (!(option instanceof HTMLButtonElement)) continue;
      option.id = "hand-labels-tag-option-" + index;
      option.dataset.handLabelTagOption = String(index);
      option.textContent = tag;
      option.setAttribute("aria-selected", String(index === selectedTag));
      tagOptions.append(option);
    }
    tagOptions.hidden = tagMatches.length === 0;
    tagInput.setAttribute("aria-expanded", String(tagMatches.length > 0));
    if (selectedTag >= 0) tagInput.setAttribute("aria-activedescendant", "hand-labels-tag-option-" + selectedTag);
    else tagInput.removeAttribute("aria-activedescendant");
  };
  const showTags = () => {
    tagValue.value = JSON.stringify(tags satisfies Wire.HandLabelTagsIn);
    tagChips.replaceChildren();
    const template = root.querySelector<HTMLTemplateElement>('[data-slot="hand-labels-tag-chip-template"]')!;
    for (const tag of tags) {
      const chip = template.content.firstElementChild?.cloneNode(true);
      if (!(chip instanceof HTMLElement)) continue;
      chip.querySelector('[data-slot="hand-labels-tag-name"]')!.textContent = tag;
      const remove = chip.querySelector<HTMLButtonElement>("[data-hand-label-remove-tag]")!;
      remove.dataset.handLabelRemoveTag = tag;
      remove.setAttribute("aria-label", "Remove tag " + tag);
      tagChips.append(chip);
    }
    tagChips.hidden = tags.length === 0;
  };
  const applyTag = (value: string) => {
    const name = value.trim().replaceAll(/\s+/g, " ");
    if (!name) return;
    const canonical = Option.getOrElse(Arr.findFirst(knownTags, tag => tagKey(tag) === tagKey(name)), () => name);
    const exists = Arr.some(tags, tag => tagKey(tag) === tagKey(canonical));
    if (Array.from(name).length > 80 || (!exists && tags.length >= 20)) {
      tagError.textContent = "Use up to 20 tags, each at most 80 characters.";
      tagError.hidden = false;
      return;
    }
    tagError.hidden = true;
    if (!exists) {
      tags.push(canonical);
      if (!knownTags.includes(canonical)) knownTags.push(canonical);
      showTags();
      void save();
    }
    tagInput.value = "";
    closeTagOptions();
    tagInput.focus({ preventScroll: true });
  };
  tagInput.addEventListener("input", () => {
    tagError.hidden = true;
    filterTags();
  }, { signal });
  tagInput.addEventListener("focus", filterTags, { signal });
  tagInput.addEventListener("blur", closeTagOptions, { signal });
  tagOptions.addEventListener("pointerdown", event => event.preventDefault(), { signal });
  tagOptions.addEventListener("click", event => {
    const option = event.target instanceof Element
      ? event.target.closest<HTMLElement>("[data-hand-label-tag-option]")
      : null;
    const name = option && tagMatches[Number(option.dataset.handLabelTagOption)];
    if (name) applyTag(name);
  }, { signal });
  tagChips.addEventListener("click", event => {
    const remove = event.target instanceof Element
      ? event.target.closest<HTMLElement>("[data-hand-label-remove-tag]")
      : null;
    if (!remove) return;
    tags = Arr.filter(tags, tag => tag !== remove.dataset.handLabelRemoveTag);
    showTags();
    void save();
    tagInput.focus({ preventScroll: true });
  }, { signal });
  const tagKeydown = (key: string) => {
    if (key === "Enter") {
      if (tagInput.value.trim() || selectedTag >= 0) applyTag(tagMatches[selectedTag] || tagInput.value);
      else if (!savedComplete || !continueNext()) moveStep(1);
      return true;
    }
    if (key === "Escape") {
      closeTagOptions();
      return true;
    }
    if (key !== "ArrowUp" && key !== "ArrowDown") return false;
    if (!tagInput.value.trim()) {
      moveStep(key === "ArrowUp" ? -1 : 1);
      return true;
    }
    if (!tagMatches.length) return true;
    if (tagOptions.hidden !== false) filterTags();
    selectedTag = selectedTag < 0
      ? (key === "ArrowDown" ? 0 : tagMatches.length - 1)
      : (selectedTag + (key === "ArrowDown" ? 1 : -1) + tagMatches.length) % tagMatches.length;
    for (const [index, option] of Array.from(tagOptions.children).entries()) {
      option.setAttribute("aria-selected", String(index === selectedTag));
    }
    const selected = tagOptions.children[selectedTag];
    tagInput.setAttribute("aria-activedescendant", selected.id);
    selected.scrollIntoView({ block: "nearest" });
    return true;
  };
  tagInput.focus({ preventScroll: true });

  let swipe: { id: number; step: HTMLElement; x: number; y: number } | undefined;
  root.addEventListener("pointerdown", (event) => {
    if (event.pointerType !== "touch") return;
    const step = event.target instanceof Element
      ? event.target.closest<HTMLElement>("fieldset[data-hand-label-step]")
      : null;
    if (step) swipe = { id: event.pointerId, step, x: event.clientX, y: event.clientY };
  }, { signal });
  root.addEventListener("pointerup", (event) => {
    if (!swipe || swipe.id !== event.pointerId) return;
    const { step, x, y } = swipe;
    swipe = undefined;
    const dx = event.clientX - x;
    if (root.inert || form()?.inert === true) return;
    if (Math.abs(dx) < 50 || Math.abs(dx) < Math.abs(event.clientY - y) * 1.5) return;
    markStep(step);
    chooseDirection(dx < 0 ? -1 : 1);
  }, { signal });
  root.addEventListener("pointercancel", () => {
    swipe = undefined;
  }, { signal });

  const unbindKeys = on('[data-slot="hand-labels"]', (event, at, from) => {
    if (at !== root) return false;
    if (event.defaultPrevented || event.isComposing || root.inert || form()?.inert === true) return false;
    const target = from instanceof HTMLIFrameElement ? from.contentDocument?.activeElement : from;
    if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
      return continueNext();
    }
    if (event.metaKey || event.ctrlKey || event.altKey) return false;
    if (target === tagInput && tagKeydown(event.key)) return true;
    if (target?.closest('textarea, input:not([type="radio"]), select, [contenteditable="true"]')) return false;
    if ((event.key === "Enter" || event.key === " ") && target?.closest("button, a[href], summary")) return false;
    if (event.key === "Enter" && !event.repeat) return continueNext();
    if (event.key === "ArrowUp" || event.key === "ArrowDown") {
      moveStep(event.key === "ArrowUp" ? -1 : 1);
      return true;
    }
    if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
      chooseDirection(event.key === "ArrowLeft" ? -1 : 1);
      return true;
    }
    if (event.key === " " && !event.repeat) {
      if (savedComplete && continueNext()) return true;
      const skip = root.querySelector<HTMLButtonElement>("[data-hand-label-skip]");
      if (!skip) return false;
      skip.click();
      return true;
    }
    for (const choice of form()!.querySelectorAll<HTMLInputElement>("[data-label-key]")) {
      if (choice.matches(":disabled")) continue;
      if (choice.dataset.labelKey === event.key.toLowerCase()) {
        choice.checked = true;
        choice.dispatchEvent(new Event("change", { bubbles: true }));
        return true;
      }
    }
    return false;
  }, { id: "hand-labels", owner: root, typing: "always", frames: true });
  signal.addEventListener("abort", unbindKeys, { once: true });
  window.addEventListener("beforeunload", (event) => {
    if (pending || dirty || blocked) {
      event.preventDefault();
      // oxlint-disable-next-line typescript/no-deprecated -- Chrome before 119 prompts only when returnValue is set
      event.returnValue = "";
    }
  }, { signal });
  signal.addEventListener("abort", () => {
    clearTimeout(timer);
    if (dirty) void save();
  }, { once: true });
});
