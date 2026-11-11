// oxlint-disable effecttsgo/async-function -- These DOM-facing APIs must retain their native Promise contracts; migrating their error channels belongs to #636.
// oxlint-disable effecttsgo/new-promise -- These bridges adapt native DOM events while preserving their Promise rejection values and event ordering.
export interface Mermaid {
  initialize: (options: {
    startOnLoad: boolean;
    securityLevel: string;
    suppressErrorRendering: boolean;
    theme: string;
  }) => void;
  render: (id: string, text: string) => Promise<{ svg: string }>;
}
declare global {
  interface Window {
    mermaid: Mermaid;
  }
}

(function() {
  let library: Promise<Mermaid> | null;
  let sequence = 0;
  let pending = Promise.resolve();

  function loadMermaid() {
    library ??= new Promise<Mermaid>((resolve, reject) => {
      const script = document.createElement("script");
      script.src = "/assets/js/mermaid.min.js";
      script.addEventListener("load", () => {
        resolve(window.mermaid);
      });
      script.addEventListener("error", () => {
        script.remove();
        library = null;
        reject(new Error("Could not load diagram renderer"));
      });
      document.head.append(script);
    });
    return library;
  }

  async function render(code: HTMLElement) {
    const pre = code.parentElement;
    if (!(pre?.isConnected === true)) return;
    try {
      const mermaid = await loadMermaid();
      // oxlint-disable-next-line typescript/no-unnecessary-condition -- Awaited rendering can detach the node.
      if (!pre.isConnected) return;
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: "strict",
        suppressErrorRendering: true,
        theme: document.documentElement.classList.contains("dark") ? "dark" : "default",
      });
      const result = await mermaid.render("file-mermaid-" + ++sequence, code.textContent);
      // oxlint-disable-next-line typescript/no-unnecessary-condition -- Awaited rendering can detach the node.
      if (!pre.isConnected) return;
      const figure = document.createElement("figure");
      figure.dataset.slot = "file-diagram";
      const diagram = document.createElement("div");
      diagram.style.overflowX = "auto";
      diagram.innerHTML = result.svg;
      const source = document.createElement("details");
      const summary = document.createElement("summary");
      summary.textContent = "Diagram source";
      source.append(summary);
      pre.replaceWith(figure);
      source.append(pre);
      figure.append(diagram, source);
    } catch {
      // oxlint-disable-next-line typescript/no-unnecessary-condition -- Awaited rendering can detach the node.
      if (!pre.isConnected) return;
      const error = document.createElement("p");
      error.dataset.slot = "file-diagram-error";
      error.setAttribute("role", "status");
      error.textContent = "Could not render this diagram. Its source is shown below.";
      pre.before(error);
    }
  }

  function scan() {
    document
      .querySelectorAll<HTMLElement>('[data-slot="files"] article pre > code.language-mermaid')
      .forEach((code) => {
        if (code.dataset.diagramState) return;
        code.dataset.diagramState = "queued";
        pending = pending.then(() => render(code));
      });
  }

  document.addEventListener("htmx:after:settle", scan);
  document.addEventListener("htmx:historyRestore", scan);
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", scan);
  else scan();
})();
