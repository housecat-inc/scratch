import { registerComponent } from "../../dom/lifecycle";

interface JSONEditor {
  execCommand: (command: string) => void;
  foldCode: (position: { line: number; ch: number }, options: null, force: "fold") => void;
  getLine: (line: number) => string;
  getWrapperElement: () => HTMLElement;
  lastLine: () => number;
  operation: (fn: () => void) => void;
  refresh: () => void;
  toTextArea: () => void;
}

interface CodeMirrorHost {
  CodeMirror?: {
    fromTextArea: (input: HTMLTextAreaElement, options: Record<string, unknown>) => JSONEditor;
  };
}

const editors = new WeakMap<HTMLTextAreaElement, JSONEditor>();
const lifetimes = new WeakMap<HTMLTextAreaElement, AbortSignal>();
const frames = new WeakMap<HTMLTextAreaElement, number>();
const selector = '[data-slot="json-viewer-input"]';

function mount(input: HTMLTextAreaElement): void {
  const signal = lifetimes.get(input);
  const codeMirror = (window as Window & CodeMirrorHost).CodeMirror;
  if (!signal || signal.aborted || !input.isConnected || !codeMirror) return;
  const existing = editors.get(input);
  if (existing) {
    existing.refresh();
    return;
  }
  // History snapshots contain the wrapper and hidden textarea, but no editor.
  const snapshot = input.nextElementSibling;
  if (snapshot?.matches('[data-slot="json-viewer-editor"]') === true) {
    snapshot.remove();
    input.style.display = "";
  }
  if (!input.getClientRects().length) return;
  const editor = codeMirror.fromTextArea(input, {
    mode: { name: "javascript", json: true },
    readOnly: true,
    lineNumbers: true,
    lineWrapping: true,
    indentUnit: 2,
    foldGutter: true,
    gutters: ["CodeMirror-linenumbers", "CodeMirror-foldgutter"],
    inputStyle: "contenteditable",
    screenReaderLabel: input.getAttribute("aria-label") ?? "JSON document",
    extraKeys: { "Ctrl-Q": "toggleFold", Tab: false, "Shift-Tab": false },
  });
  editors.set(input, editor);
  editor.getWrapperElement().setAttribute("data-slot", "json-viewer-editor");
  editor.operation(() => {
    for (let line = 1; line < editor.lastLine(); line++) {
      if (/^  "(?:[^"\\]|\\.)*": [[{]/.test(editor.getLine(line))) {
        editor.foldCode({ line, ch: 0 }, null, "fold");
      }
    }
  });
  frames.set(
    input,
    requestAnimationFrame(() => {
      frames.delete(input);
      if (!signal.aborted) editor.refresh();
    }),
  );
  void document.fonts.ready.then(() => {
    if (!signal.aborted) editor.refresh();
  });
}

registerComponent(selector, (input, signal) => {
  if (!(input instanceof HTMLTextAreaElement)) return;
  lifetimes.set(input, signal);
  signal.addEventListener("abort", () => {
    const frame = frames.get(input);
    if (frame !== undefined) cancelAnimationFrame(frame);
    frames.delete(input);
    lifetimes.delete(input);
    const editor = editors.get(input);
    editors.delete(input);
    if (editor) {
      const wrapper = editor.getWrapperElement();
      // CodeMirror expects its textarea and wrapper to share a parent, even
      // when a swap removed the textarea alone. Dispose off-document then.
      if (!input.parentNode || input.parentNode !== wrapper.parentNode) {
        const detached = document.createDocumentFragment();
        detached.append(input, wrapper);
        editor.toTextArea();
        input.remove();
      } else editor.toTextArea();
    }
  }, { once: true });
  mount(input);
});

document.addEventListener("click", (event) => {
  const button = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-json-action]") : null;
  const input = button?.closest('[data-slot="json-viewer"]')?.querySelector<HTMLTextAreaElement>(selector);
  if (!input) return;
  const command = button?.dataset.jsonAction === "fold" ? "foldAll" : "unfoldAll";
  editors.get(input)?.execCommand(command);
});

document.addEventListener("toggle", (event) => {
  if (event.target instanceof HTMLDetailsElement && event.target.open) {
    const details = event.target;
    requestAnimationFrame(() => details.querySelectorAll<HTMLTextAreaElement>(selector).forEach(mount));
  }
}, true);

// CodeMirror may load after this module; closed details mount on their first toggle.
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => {
    document.querySelectorAll<HTMLTextAreaElement>(selector).forEach(mount);
  }, { once: true });
}
