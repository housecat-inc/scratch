import * as Arr from "effect/Array";
import * as Str from "effect/String";
import * as Wire from "../schemas/schemas.gen";
import { readJSON, readResponseJSON, reportUnlessAborted } from "./failures";
import { registerComponent } from "./lifecycle";
import { isErrorMessage } from "./request_outcome";
import { readDecoded, type WireContext } from "./wire";

registerComponent('[data-slot="hand-labels-labeled"][data-inline-review="true"]', (root, signal) => {
  type ReviewState = {
    base: string;
    dirty: boolean;
    form: HTMLFormElement;
    tags: string[];
    timer?: ReturnType<typeof setTimeout>;
  };
  const states = new Map<HTMLFormElement, ReviewState>();
  const failed = new Set<Element>();
  let queue = Promise.resolve();
  let pending = 0;
  let navigating = false;
  const values = (form: HTMLFormElement) => {
    const data = new URLSearchParams();
    for (const [key, value] of new FormData(form)) if (typeof value === "string") data.append(key, value);
    return data;
  };
  const text = (target: Element, selector: string, message: string) => {
    const el = target.querySelector(selector);
    if (el) el.textContent = message;
  };
  const post = async (form: HTMLFormElement, data: URLSearchParams) => {
    const res = await fetch(form.action, { method: "POST", body: data, keepalive: true });
    if (!res.ok) {
      const body = await readResponseJSON(res);
      const error = new Error(
        isErrorMessage(body) ? body.message : "Could not confirm the save. Check your connection and retry.",
      );
      if (res.status === 409) error.name = "ConflictError";
      throw error;
    }
    return res;
  };
  const refreshFilters = async () => {
    try {
      const res = await fetch(location.href, { signal });
      if (!res.ok) return;
      const page = new DOMParser().parseFromString(await res.text(), "text/html");
      const next = page.querySelector('[data-slot="hand-labels-filters"]');
      const current = root.querySelector('[data-slot="hand-labels-filters"]');
      if (next && current && !signal.aborted) current.replaceWith(document.importNode(next, true));
    } catch (failure) {
      reportUnlessAborted({ operation: "hand-labels.refresh-filters" })(failure);
    }
  };
  const enqueue = (target: Element, statusSelector: string, operation: () => Promise<void>) => {
    pending++;
    text(target, statusSelector, "Saving…");
    queue = queue.then(async () => {
      try {
        await operation();
        failed.delete(target);
        if (!signal.aborted) {
          text(
            target,
            statusSelector,
            target instanceof HTMLFormElement && states.get(target)?.dirty === true ? "Saving…" : "Saved",
          );
        }
      } catch (error) {
        failed.add(target);
        if (!signal.aborted) {
          text(target, statusSelector, error instanceof Error ? error.message : "Could not save. Retry or reload.");
          const retry = target.querySelector<HTMLElement>("[data-failure-retry]");
          const conflict = error instanceof Error && error.name === "ConflictError";
          if (retry) retry.hidden = conflict;
          const reload = target.querySelector<HTMLElement>("[data-failure-reload]");
          if (reload) reload.hidden = !conflict;
        }
      } finally {
        pending--;
        if (target instanceof HTMLElement) target.inert = false;
      }
    });
    return queue;
  };
  const saveReview = (state: ReviewState) => {
    clearTimeout(state.timer);
    state.dirty = false;
    const data = values(state.form);
    return enqueue(state.form, "[data-failure-status]", async () => {
      // Each queued write uses the preceding save's revision, not its old form snapshot.
      data.set("base", state.base);
      const res = await post(state.form, data);
      const saved = readDecoded(
        Wire.FailureReviewOut,
        { endpoint: "/evals/:id/failure-review", operation: "hand-labels.save-review" },
        await res.json(),
      );
      state.base = String(saved.revision);
      if (!signal.aborted) {
        const retry = state.form.querySelector<HTMLElement>("[data-failure-retry]");
        if (retry) retry.hidden = true;
        await refreshFilters();
      }
    });
  };
  // parseTags reads failure tags as the form renders them (app/ui/hand_labels'
  // failureTagsJSON): the tags the workspace knows, and the ones a review has.
  const parseTags = (raw: string): string[] => {
    const parsed = readJSON(raw);
    if (parsed === undefined) return [];
    const rendered: WireContext = { endpoint: "/evals/:id", operation: "hand-labels.failure-tags" };
    return [...(readDecoded(Wire.FailureTags, rendered, parsed) ?? [])];
  };
  const known: string[] = [];
  for (const form of root.querySelectorAll<HTMLFormElement>('[data-slot="hand-label-failure-form"]')) {
    const hidden = form.querySelector<HTMLInputElement>('input[name="failure_tags"]')!;
    const input = form.querySelector<HTMLInputElement>("[data-failure-tag-input]")!;
    const list = form.querySelector<HTMLElement>("[data-failure-suggestions]")!;
    const chips = form.querySelector<HTMLElement>("[data-failure-tags]")!;
    for (const tag of parseTags(form.dataset.failureKnown ?? "[]")) if (!Arr.contains(known, tag)) known.push(tag);
    const state: ReviewState = {
      base: form.dataset.revision ?? "0",
      dirty: false,
      form,
      tags: parseTags(hidden.value),
    };
    states.set(form, state);
    let matches: string[] = [];
    let selected = 0;
    const suggest = () => {
      const query = Str.toLowerCase(Str.trim(input.value));
      matches = Arr.take(
        Arr.filter(known, tag =>
          !Arr.some(state.tags, value => Str.toLowerCase(value) === Str.toLowerCase(tag))
          && (query.length === 0 || Str.includes(query)(Str.toLowerCase(tag)))),
        10,
      );
      selected = Math.max(0, Math.min(selected, matches.length - 1));
      list.replaceChildren();
      list.hidden = document.activeElement !== input || matches.length === 0;
      input.setAttribute("aria-expanded", String(!list.hidden));
      input.removeAttribute("aria-activedescendant");
      Arr.forEach(matches, (tag, index) => {
        const option = document.createElement("button");
        option.type = "button";
        option.id = list.id + "-" + index;
        option.setAttribute("role", "option");
        option.setAttribute("aria-selected", String(index === selected));
        option.className =
          "block w-full rounded px-2 py-1.5 text-left type-body-100 hover:bg-muted aria-selected:bg-muted";
        option.textContent = tag;
        option.addEventListener("mousedown", event => event.preventDefault(), { signal });
        option.addEventListener("click", () => add(tag), { signal });
        list.append(option);
        if (index === selected && list.hidden === false) input.setAttribute("aria-activedescendant", option.id);
      });
    };
    const renderTags = () => {
      hidden.value = JSON.stringify(state.tags);
      chips.replaceChildren();
      for (const tag of state.tags) {
        const chip = document.createElement("button");
        chip.type = "button";
        chip.className = "rounded-md bg-muted px-2 py-1.5 type-caption-100 hover:bg-muted/70";
        chip.textContent = tag + " ×";
        chip.setAttribute("aria-label", "Remove failure tag " + tag);
        chip.addEventListener("click", () => {
          state.tags = Arr.filter(state.tags, value => value !== tag);
          renderTags();
          void saveReview(state);
        }, { signal });
        chips.append(chip);
      }
    };
    const add = (tag: string) => {
      tag = Str.replaceAll(/\s+/g, " ")(Str.trim(tag));
      if (tag.length === 0) return;
      if (!Arr.some(state.tags, value => Str.toLowerCase(value) === Str.toLowerCase(tag))) state.tags.push(tag);
      if (!Arr.some(known, value => Str.toLowerCase(value) === Str.toLowerCase(tag))) known.push(tag);
      input.value = "";
      renderTags();
      void saveReview(state);
      input.focus({ preventScroll: true });
      suggest();
    };
    renderTags();
    input.addEventListener("focus", suggest, { signal });
    input.addEventListener("input", () => {
      selected = 0;
      suggest();
    }, { signal });
    input.addEventListener("blur", () => {
      list.hidden = true;
      input.setAttribute("aria-expanded", "false");
    }, { signal });
    input.addEventListener("keydown", event => {
      if (event.isComposing) return;
      if (event.key === "Enter") {
        event.preventDefault();
        if (Str.trim(input.value).length > 0) add(matches[selected] ?? input.value);
      } else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        selected += event.key === "ArrowDown" ? 1 : -1;
        suggest();
      } else if (event.key === "Escape") {
        list.hidden = true;
        input.setAttribute("aria-expanded", "false");
      }
    }, { signal });
    form.addEventListener("input", event => {
      if (!(event.target instanceof HTMLTextAreaElement)) return;
      state.dirty = true;
      text(form, "[data-failure-status]", "Saving…");
      clearTimeout(state.timer);
      state.timer = setTimeout(() => {
        void saveReview(state);
      }, 500);
    }, { signal });
    form.addEventListener("focusout", event => {
      if (state.dirty && event.target instanceof HTMLTextAreaElement) void saveReview(state);
    }, { signal });
  }
  const saveAxis = (form: HTMLFormElement) => {
    const comparison = form.closest<HTMLElement>('[data-slot="hand-label-comparison"]')!;
    const data = values(form);
    comparison.inert = true;
    void enqueue(comparison, "[data-axis-save-status]", async () => {
      const res = await post(form, data);
      const page = new DOMParser().parseFromString(await res.text(), "text/html");
      const replacement = page.querySelector<HTMLElement>('[data-slot="hand-label-comparison"]');
      if (!replacement) throw new Error("Could not refresh your saved labels. Reload this page.");
      if (!signal.aborted) {
        comparison.replaceWith(document.importNode(replacement, true));
        await refreshFilters();
      }
    });
  };
  root.addEventListener("change", event => {
    if (!(event.target instanceof HTMLSelectElement)) return;
    const form = event.target.closest<HTMLFormElement>('[data-slot="hand-label-axis-form"]');
    if (form) saveAxis(form);
  }, { signal });
  const flush = async () => {
    for (const state of states.values()) if (state.dirty) void saveReview(state);
    await queue;
    return failed.size === 0;
  };
  root.addEventListener("submit", event => {
    if (!(event.target instanceof HTMLFormElement)) return;
    const form = event.target;
    const state = states.get(form);
    if (state) {
      event.preventDefault();
      void saveReview(state);
    } else if (form.matches('[data-slot="hand-label-axis-form"]')) {
      event.preventDefault();
      saveAxis(form);
    } else if (pending > 0 || failed.size > 0 || Arr.some(Arr.fromIterable(states.values()), item => item.dirty)) {
      event.preventDefault();
      const submitter = event.submitter;
      if (navigating) return;
      navigating = true;
      root.inert = true;
      void flush().then(ok => {
        if (ok && !signal.aborted) form.requestSubmit(submitter);
        else {
          navigating = false;
          root.inert = false;
        }
      });
    }
  }, { signal });
  document.addEventListener("click", event => {
    if (!(event.target instanceof Element) || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey) {
      return;
    }
    const link = event.target.closest<HTMLAnchorElement>("a[href]");
    if (!link || link.target.length > 0 || link.hasAttribute("data-failure-reload")) return;
    if (pending === 0 && failed.size === 0 && !Arr.some(Arr.fromIterable(states.values()), item => item.dirty)) return;
    event.preventDefault();
    if (navigating) return;
    navigating = true;
    root.inert = true;
    void flush().then(ok => {
      if (ok && !signal.aborted) location.assign(link.href);
      else {
        navigating = false;
        root.inert = false;
      }
    });
  }, { signal });
  window.addEventListener("beforeunload", event => {
    if (pending > 0 || failed.size > 0 || Arr.some(Arr.fromIterable(states.values()), state => state.dirty)) {
      event.preventDefault();
    }
  }, { signal });
  signal.addEventListener("abort", () => {
    for (const state of states.values()) clearTimeout(state.timer);
  }, { once: true });
});
