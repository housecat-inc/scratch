// oxlint-disable effecttsgo/async-function -- Uploads and html2canvas are native Promise APIs.
// oxlint-disable effecttsgo/global-fetch -- Uploads read the server's HTML chip straight off the Response.
import { registerComponent } from "../../dom/lifecycle";

declare global {
  interface Window {
    html2canvas?: (el: HTMLElement, options: Record<string, unknown>) => Promise<HTMLCanvasElement>;
  }
}

const html2canvasURL = "/assets/html2canvas.min.js";
let html2canvasLoad: Promise<void> | undefined;

function alert(message: string): void {
  window.HousecatToast?.show({ message, duration: 6000 });
}

function loadHtml2canvas(): Promise<void> {
  html2canvasLoad ??= new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = html2canvasURL;
    script.onload = () => resolve();
    script.onerror = () => {
      html2canvasLoad = undefined;
      reject(new Error("could not load html2canvas"));
    };
    document.head.append(script);
  });
  return html2canvasLoad;
}

export function attachmentIDs(form: Element): string[] {
  return Array.from(
    form.closest("[data-chat-composer]")?.querySelectorAll<HTMLElement>("[data-chat-attachments] [data-attachment-id]") ?? [],
    (chip) => chip.dataset.attachmentId ?? "",
  ).filter(Boolean);
}

export function clearAttachments(form: Element): void {
  form.closest("[data-chat-composer]")?.querySelector("[data-chat-attachments]")?.replaceChildren();
}

function cssPath(el: Element): string {
  const parts: string[] = [];
  for (let node: Element | null = el; node && parts.length < 6; node = node.parentElement) {
    if (node.id) {
      parts.unshift("#" + node.id);
      break;
    }
    let part = node.tagName.toLowerCase();
    const classes = Array.from(node.classList).filter((c) => /^[\w-]+$/.test(c)).slice(0, 2).join(".");
    if (classes) part += "." + classes;
    const siblings = node.parentElement
      ? Array.from(node.parentElement.children).filter((c) => c.tagName === node!.tagName)
      : [];
    if (siblings.length > 1) part += `:nth-of-type(${siblings.indexOf(node) + 1})`;
    parts.unshift(part);
  }
  return parts.join(" > ");
}

function sectionOf(el: Element): Element {
  const preferred = el.closest(
    "[data-turn-id], article, section, aside, nav, form, header, footer, li, tr",
  );
  if (preferred && preferred !== document.body) return preferred;
  let node = el;
  while (
    node.parentElement
    && node.parentElement !== document.body
    && (node.getBoundingClientRect().width < 80 || node.getBoundingClientRect().height < 32)
  ) {
    node = node.parentElement;
  }
  return node;
}

registerComponent("[data-chat-composer][data-attach-url]", (root, signal) => {
  const url = root.dataset.attachUrl ?? "";
  const strip = root.querySelector<HTMLElement>("[data-chat-attachments]");
  const fileInput = root.querySelector<HTMLInputElement>("[data-chat-file]");
  const field = root.querySelector<HTMLTextAreaElement>("[data-prompt-field]");
  const form = root.querySelector<HTMLFormElement>("[data-chat-form]");
  if (!strip || !fileInput || !field || !form) return undefined;

  async function upload(file: File): Promise<void> {
    const body = new FormData();
    body.append("file", file, file.name || "pasted-image.png");
    try {
      const res = await fetch(url, { body, method: "POST" });
      if (!res.ok) {
        alert(await res.text());
        return;
      }
      strip!.insertAdjacentHTML("beforeend", await res.text());
    } catch (error) {
      alert("Upload failed: " + (error instanceof Error ? error.message : String(error)));
    }
  }

  async function uploadAll(files: Iterable<File>): Promise<void> {
    for (const file of Array.from(files)) await upload(file);
  }

  function appendText(text: string): void {
    field!.value = field!.value ? field!.value + "\n" + text : text;
    field!.dispatchEvent(new Event("input", { bubbles: true }));
    root.querySelector<HTMLElement>("[data-slot=chat-composer-editor]")?.focus({ preventScroll: true });
    field!.focus({ preventScroll: true });
  }

  async function capture(el: Element): Promise<void> {
    const selector = cssPath(el);
    const files: File[] = [];
    try {
      await loadHtml2canvas();
      const canvas = await window.html2canvas!(el as HTMLElement, { logging: false, useCORS: true });
      const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/png"));
      if (blob) files.push(new File([blob], "selection.png", { type: "image/png" }));
    } catch (error) {
      alert("Screenshot failed: " + (error instanceof Error ? error.message : String(error)));
    }
    const html = "<!-- url: " + location.href + " -->\n<!-- selector: " + selector + " -->\n" + el.outerHTML;
    files.push(new File([html], "selection.html", { type: "text/html" }));
    await uploadAll(files);
    appendText("Selected " + selector + " on " + location.href);
  }

  function inspect(): void {
    if (document.body.dataset.chatInspecting === "true") return;
    document.body.dataset.chatInspecting = "true";
    const overlay = document.createElement("div");
    overlay.className = "pointer-events-none fixed z-[9999] hidden rounded-sm border-2 border-primary bg-primary/10";
    const label = document.createElement("span");
    label.className = "absolute -top-6 left-0 max-w-[80vw] truncate rounded-sm bg-primary px-1.5 py-0.5 font-mono text-[11px] text-primary-foreground";
    overlay.append(label);
    document.body.append(overlay);
    let target: Element | null = null;

    const stop = () => {
      target = null;
      overlay.remove();
      delete document.body.dataset.chatInspecting;
      document.removeEventListener("mousemove", move, true);
      document.removeEventListener("click", pick, true);
      document.removeEventListener("keydown", key, true);
    };
    const move = (event: MouseEvent) => {
      const el = document.elementFromPoint(event.clientX, event.clientY);
      if (!el || el === document.body || el === document.documentElement || el.closest("[data-chat-composer]")) {
        target = null;
        overlay.classList.add("hidden");
        return;
      }
      target = sectionOf(el);
      const rect = target.getBoundingClientRect();
      overlay.classList.remove("hidden");
      Object.assign(overlay.style, {
        height: rect.height + "px",
        left: rect.left + "px",
        top: rect.top + "px",
        width: rect.width + "px",
      });
      label.classList.toggle("-top-6", rect.top >= 30);
      label.classList.toggle("top-0", rect.top < 30);
      label.textContent = cssPath(target) + "  " + Math.round(rect.width) + "×" + Math.round(rect.height);
    };
    const key = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      stop();
    };
    const pick = (event: MouseEvent) => {
      event.preventDefault();
      event.stopPropagation();
      const el = target;
      stop();
      if (el) void capture(el);
    };
    document.addEventListener("mousemove", move, true);
    setTimeout(() => {
      document.addEventListener("click", pick, true);
      document.addEventListener("keydown", key, true);
    }, 0);
  }

  root.addEventListener("click", (event) => {
    const target = event.target instanceof Element ? event.target : null;
    if (target?.closest("[data-chat-attach]")) {
      fileInput.click();
      return;
    }
    if (target?.closest("[data-chat-inspect]")) {
      inspect();
      return;
    }
    const remove = target?.closest<HTMLElement>("[data-attachment-remove]");
    const chip = remove?.closest<HTMLElement>("[data-attachment-id]");
    if (!chip) return;
    event.preventDefault();
    chip.remove();
    void fetch(url + "/" + chip.dataset.attachmentId, { method: "DELETE" });
  }, { signal });

  fileInput.addEventListener("change", () => {
    const files = Array.from(fileInput.files ?? []);
    fileInput.value = "";
    void uploadAll(files);
  }, { signal });

  root.addEventListener("paste", (event) => {
    if (!event.clipboardData?.files.length) return;
    event.preventDefault();
    event.stopPropagation();
    void uploadAll(event.clipboardData.files);
  }, { signal });

  form.addEventListener("dragover", (event) => {
    if (!event.dataTransfer?.types.includes("Files")) return;
    event.preventDefault();
    form.dataset.dropTarget = "true";
  }, { signal });
  form.addEventListener("dragleave", () => {
    delete form.dataset.dropTarget;
  }, { signal });
  form.addEventListener("drop", (event) => {
    delete form.dataset.dropTarget;
    if (!event.dataTransfer?.files.length) return;
    event.preventDefault();
    void uploadAll(event.dataTransfer.files);
  }, { signal });

  return undefined;
});
