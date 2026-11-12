import { composing } from "../../dom/keyinfo";
import { on } from "../../dom/keys";
import { readDecoded } from "../../dom/wire";
import * as Wire from "../../schemas/schemas.gen";

const editorSelector = '[data-slot="chat-composer-editor"]';
const chipSelector = '[data-slot="chat-composer-link"]';
const editors = new WeakMap<HTMLTextAreaElement, HTMLElement>();
const labels = new WeakMap<HTMLElement, Map<string, Promise<string>>>();

type Position = { path: number[]; offset: number };
type Snapshot = { html: string; start?: Position; end?: Position };
const histories = new WeakMap<HTMLElement, { undo: Snapshot[]; redo: Snapshot[] }>();

function snapshot(editor: HTMLElement): Snapshot {
  editor.normalize();
  const result: Snapshot = { html: editor.innerHTML };
  const selection = getSelection();
  if (!selection || selection.rangeCount === 0) return result;
  const range = selection.getRangeAt(0);
  function position(node: Node, offset: number): Position | undefined {
    if (!editor.contains(node)) return undefined;
    const path: number[] = [];
    while (node !== editor) {
      const parent = node.parentNode!;
      path.unshift(Array.from<Node>(parent.childNodes).indexOf(node));
      node = parent;
    }
    return { path, offset };
  }
  const start = position(range.startContainer, range.startOffset);
  const end = position(range.endContainer, range.endOffset);
  if (start && end) {
    result.start = start;
    result.end = end;
  }
  return result;
}

function remember(editor: HTMLElement): void {
  const history = histories.get(editor)!;
  history.undo.push(snapshot(editor));
  if (history.undo.length > 100) history.undo.shift();
  history.redo = [];
}

function restore(editor: HTMLElement, redo: boolean): void {
  const history = histories.get(editor)!;
  const target = (redo ? history.redo : history.undo).pop();
  if (!target) return;
  (redo ? history.undo : history.redo).push(snapshot(editor));
  editor.innerHTML = target.html;
  editor.focus();
  const range = document.createRange();
  range.selectNodeContents(editor);
  range.collapse(false);
  function locate(position: Position): Node {
    return position.path.reduce<Node>((node, index) => node.childNodes[index], editor);
  }
  if (target.start && target.end) {
    range.setStart(locate(target.start), target.start.offset);
    range.setEnd(locate(target.end), target.end.offset);
  }
  getSelection()?.removeAllRanges();
  getSelection()?.addRange(range);
  sync(editor.closest("form")!.querySelector<HTMLTextAreaElement>("textarea")!, editor);
  enrich(editor);
}

function linkURL(text: string): URL | null {
  const value = text.trim();
  if (/\s/.test(value) || value.length > 4096) return null;
  if (!/^(https?:\/\/|www\.|\/mail(?:[/?#]|$))/i.test(value)) return null;
  try {
    const url = new URL(value.toLowerCase().startsWith("www.") ? "https://" + value : value, location.origin);
    if (url.protocol !== "http:" && url.protocol !== "https:") return null;
    if (url.username || url.password) return null;
    return url;
  } catch {
    return null;
  }
}

function plainText(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? "";
  if (node instanceof HTMLElement && node.matches(chipSelector)) return node.dataset.url ?? "";
  let text = "";
  for (const child of node.childNodes) {
    if (child instanceof HTMLBRElement) {
      if (child.nextSibling || !(node instanceof HTMLElement)) text += "\n";
    } else {
      if (child instanceof HTMLElement && /^(DIV|P)$/.test(child.tagName) && child.previousSibling) text += "\n";
      text += plainText(child);
    }
  }
  return text;
}

function sync(field: HTMLTextAreaElement, editor: HTMLElement): void {
  field.value = plainText(editor);
  field.dispatchEvent(new Event("input", { bubbles: true }));
}

function fallback(url: string): string {
  const parsed = new URL(url.toLowerCase().startsWith("www.") ? "https://" + url : url, location.origin);
  return parsed.hostname + (parsed.pathname === "/" ? "" : parsed.pathname);
}

function enrich(editor: HTMLElement): void {
  const cache = labels.get(editor)!;
  for (const chip of editor.querySelectorAll<HTMLElement>(chipSelector)) {
    const url = chip.dataset.url!;
    if (!cache.has(url)) {
      if (cache.size >= 50) continue;
      cache.set(
        url,
        (async () => {
          const controller = new AbortController();
          const timeout = window.setTimeout(() => controller.abort(), 2000);
          try {
            const response = await fetch("/codex/link-preview?url=" + encodeURIComponent(url), {
              signal: controller.signal,
            });
            if (!response.ok) return fallback(url);
            const label = readDecoded(
              Wire.LinkPreviewOut,
              { endpoint: "/codex/link-preview", operation: "chat.composer-link-label" },
              await response.json(),
            );
            if (!label.label) return fallback(url);
            return label.label + (label.context ? " · " + label.context : "");
          } catch {
            return fallback(url);
          } finally {
            clearTimeout(timeout);
          }
        })(),
      );
    }
    void cache.get(url)!.then(label => {
      if (editor.contains(chip) && chip.dataset.label !== label) {
        chip.dataset.label = label;
        chip.setAttribute("aria-label", label);
        chip.textContent = label;
      }
    });
  }
}

function insert(editor: HTMLElement, text: string): void {
  const field = editor.closest("form")!.querySelector<HTMLTextAreaElement>("textarea")!;
  const selected = window.getSelection();
  if (
    !selected || selected.rangeCount === 0 || !editor.contains(selected.anchorNode)
    || !editor.contains(selected.focusNode)
  ) return;
  const fragment = selected.getRangeAt(0).cloneContents();
  if (field.value.length - plainText(fragment).length + text.length > field.maxLength) return;
  remember(editor);
  const range = selected.getRangeAt(0);
  range.deleteContents();
  const fragmentToInsert = document.createDocumentFragment();
  const destination = linkURL(text);
  if (destination && (destination.protocol === "https:" || destination.protocol === "http:")) {
    const url = text.trim();
    const chip = document.createElement("a");
    chip.dataset.slot = "chat-composer-link";
    chip.dataset.url = url;
    chip.contentEditable = "false";
    chip.title = url;
    chip.href = destination.href;
    chip.target = "_blank";
    chip.rel = "noopener noreferrer";
    chip.className =
      "inline-block max-w-full truncate rounded-md border border-border bg-muted px-1.5 align-middle type-body-200 text-primary";
    chip.dataset.label = fallback(url);
    chip.textContent = fallback(url);
    const offset = text.indexOf(url);
    fragmentToInsert.append(
      document.createTextNode(text.slice(0, offset)),
      chip,
      document.createTextNode(text.slice(offset + url.length)),
    );
  } else {
    fragmentToInsert.append(document.createTextNode(text));
  }
  const last = fragmentToInsert.lastChild!;
  range.insertNode(fragmentToInsert);
  range.setStartAfter(last);
  range.collapse(true);
  selected.removeAllRanges();
  selected.addRange(range);
  if (text.endsWith("\n") && !last.nextSibling) editor.append(document.createElement("br"));
  sync(field, editor);
  enrich(editor);
}

function enhance(field: HTMLTextAreaElement): HTMLElement {
  const existing = editors.get(field);
  if (existing) return existing;
  const start = field.selectionStart;
  const end = field.selectionEnd;
  const editor = document.createElement("div");
  editor.dataset.slot = "chat-composer-editor";
  editor.contentEditable = "true";
  editor.setAttribute("role", "textbox");
  editor.setAttribute("aria-multiline", "true");
  editor.setAttribute("aria-label", field.getAttribute("aria-label") ?? "Message");
  editor.dataset.placeholder = field.placeholder;
  editor.className = field.className;
  editor.classList.add(
    "whitespace-pre-wrap",
    "break-words",
    "overflow-y-auto",
    "empty:before:content-[attr(data-placeholder)]",
    "empty:before:text-muted-foreground",
  );
  editor.style.maxHeight = "160px";
  editor.textContent = field.value;
  field.before(editor);
  field.hidden = true;
  field.required = false;
  editors.set(field, editor);
  labels.set(editor, new Map());
  histories.set(editor, { undo: [], redo: [] });
  editor.addEventListener("input", () => {
    sync(field, editor);
    enrich(editor);
  });
  field.addEventListener("input", () => {
    if (field.value !== plainText(editor)) {
      editor.textContent = field.value;
      histories.set(editor, { undo: [], redo: [] });
    }
  });
  editor.addEventListener("compositionstart", () => remember(editor));
  // The key router leaves editing chords to the focused editor's history.
  editor.addEventListener("keydown", event => {
    if (field.disabled || field.readOnly || composing(event)) return;
    if (!(event.ctrlKey || event.metaKey) || event.altKey || !["z", "y"].includes(event.key.toLowerCase())) return;
    event.preventDefault();
    event.stopPropagation();
    restore(editor, event.shiftKey || event.key.toLowerCase() === "y");
  });
  editor.addEventListener("beforeinput", event => {
    if (field.disabled || field.readOnly) {
      event.preventDefault();
      return;
    }
    if (event.inputType === "historyUndo" || event.inputType === "historyRedo") {
      event.preventDefault();
      restore(editor, event.inputType === "historyRedo");
      return;
    }

    if (event.inputType === "insertLineBreak" || event.inputType === "insertParagraph") {
      event.preventDefault();
      insert(editor, "\n");
      return;
    }
    const selection = getSelection();
    const replaced = selection && selection.rangeCount > 0
      ? plainText(selection.getRangeAt(0).cloneContents()).length
      : 0;
    const added = event.data?.length
      ?? (event.inputType === "insertParagraph" || event.inputType === "insertLineBreak" ? 1 : 0);
    if (event.inputType.startsWith("insert") && field.value.length - replaced + added > field.maxLength) {
      event.preventDefault();
      return;
    }
    if (!event.isComposing) remember(editor);
  });
  for (const type of ["copy", "cut"] as const) {
    editor.addEventListener(type, event => {
      const selection = window.getSelection();
      if (!selection || selection.rangeCount === 0 || selection.isCollapsed || !event.clipboardData) return;
      event.preventDefault();
      event.clipboardData.setData("text/plain", plainText(selection.getRangeAt(0).cloneContents()));
      if (type === "cut" && !field.disabled && !field.readOnly) {
        remember(editor);
        selection.getRangeAt(0).deleteContents();
        sync(field, editor);
      }
    });
  }
  editor.addEventListener("drop", event => {
    event.preventDefault();
  });
  editor.focus();
  const text = editor.firstChild;
  if (text) {
    const range = document.createRange();
    range.setStart(text, start);
    range.setEnd(text, end);
    window.getSelection()?.removeAllRanges();
    window.getSelection()?.addRange(range);
  }
  return editor;
}

document.addEventListener("paste", event => {
  const target = event.target;
  if (!(target instanceof Element)) return;
  const field = target.closest("[data-chat-composer]")?.querySelector<HTMLTextAreaElement>("textarea");
  if (!field || field.disabled || field.readOnly || !event.clipboardData) return;
  let editor = target.closest<HTMLElement>(editorSelector);
  if (target !== field && !editor) return;
  const text = event.clipboardData.getData("text/plain").replaceAll(/\r\n?/g, "\n");
  if (!editor && !linkURL(text)) return;
  event.preventDefault();
  editor ??= enhance(field);
  insert(editor, text);
});

on(editorSelector, (event, editor) => {
  const field = editor.closest("form")!.querySelector<HTMLTextAreaElement>("textarea")!;
  if (field.disabled || field.readOnly || composing(event)) return false;

  if (event.key !== "Enter" || event.shiftKey || event.ctrlKey || event.metaKey || event.altKey) return false;
  const submit = field.form!.querySelector<HTMLButtonElement>("[data-prompt-submit]")!;
  if (!submit.disabled && field.value.trim()) field.form!.requestSubmit(submit);
  return true;
}, { typing: "always" });
