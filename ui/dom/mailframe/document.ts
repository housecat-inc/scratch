const OPEN_LABEL = "Show trimmed content";
const CLOSE_LABEL = "Hide trimmed content";
const QUOTE_STYLE =
  ".gm-quote-section{display:block;margin-top:4px}.gm-quote-toggle{align-items:center;background:transparent;border:0;border-radius:4px;color:#5f6368;cursor:pointer;display:inline-flex;font:600 14px/1 Roboto,Arial,sans-serif;height:20px;justify-content:center;letter-spacing:0;margin:0;padding:0;width:16px}.gm-quote-toggle:hover,.gm-quote-toggle.on{color:#202124}.gm-quote-content{display:block;min-width:0}.gm-quote-content .gmail_quote,.gm-quote-content blockquote,.gm-quote-content blockquote[type='cite']{border-left:0!important;margin-left:0!important;padding-left:0!important}.gm-quote-content[hidden]{display:none!important}";

// findQuoteNodes returns the top-level quoted or trimmed blocks.
function findQuoteNodes(doc: Document): Element[] {
  const set = new Set<Element>();
  doc
    .querySelectorAll(
      '.gmail_quote, blockquote[type="cite"], blockquote, .gmail_extra, .yahoo_quoted, .protonmail_quote',
    )
    .forEach((n) => set.add(n));
  const app = doc.querySelector("#appendonsend");
  for (let n = app?.nextElementSibling; n; n = n.nextElementSibling) set.add(n);
  const fwd = doc.querySelector('#divRplyFwdMsg, [id^="divRplyFwdMsg"]');
  for (let n: Element | null = fwd; n; n = n.nextElementSibling) set.add(n);
  return Array.from(set).filter((n) => {
    for (let p = n.parentElement; p; p = p.parentElement) if (set.has(p)) return false;
    return true;
  });
}

function wireQuoteToggle(refit: () => void, btn: HTMLElement | null, content: HTMLElement | null): void {
  if (!btn || !content || btn.dataset.quoteWired === "true") return;
  btn.dataset.quoteWired = "true";
  btn.onclick = () => {
    const show = btn.classList.toggle("on");
    btn.toggleAttribute("data-body-open", show);
    content.hidden = !show;
    btn.title = show
      ? btn.getAttribute("data-quote-close-label") || CLOSE_LABEL
      : btn.getAttribute("data-quote-open-label") || OPEN_LABEL;
    btn.setAttribute("aria-label", btn.title);
    refit();
  };
}

export function prepareDocument(doc: Document, refit: () => void): void {
  doc.querySelectorAll("a[href]").forEach((a) => {
    a.setAttribute("target", "_blank");
    a.setAttribute("rel", "noopener noreferrer");
  });
  if (!doc.querySelector("[data-gm-quote-style]")) {
    const style = doc.createElement("style");
    style.setAttribute("data-gm-quote-style", "1");
    style.textContent = QUOTE_STYLE;
    // A document that isn't HTML has no head, which TypeScript's DOM types leave out.
    ((doc.head as HTMLHeadElement | null) ?? doc.documentElement).appendChild(style);
  }
  doc.querySelectorAll<HTMLElement>(".gm-quote-toggle").forEach((btn) => {
    wireQuoteToggle(refit, btn, btn.parentElement?.querySelector<HTMLElement>(".gm-quote-content") ?? null);
  });
  const quotes = findQuoteNodes(doc);
  if (quotes.length && !doc.querySelector(".gm-quote-toggle")) {
    const section = doc.createElement("div");
    section.className = "gm-quote-section";
    const btn = doc.createElement("button");
    btn.type = "button";
    btn.className = "gm-quote-toggle";
    btn.textContent = "...";
    btn.title = OPEN_LABEL;
    btn.setAttribute("aria-label", OPEN_LABEL);
    btn.setAttribute("data-quote-open-label", OPEN_LABEL);
    btn.setAttribute("data-quote-close-label", CLOSE_LABEL);
    const content = doc.createElement("div");
    content.className = "gm-quote-content";
    content.hidden = true;
    wireQuoteToggle(refit, btn, content);
    quotes[0].parentNode!.insertBefore(section, quotes[0]);
    section.appendChild(btn);
    section.appendChild(content);
    quotes.forEach((n) => content.appendChild(n));
  }
}
