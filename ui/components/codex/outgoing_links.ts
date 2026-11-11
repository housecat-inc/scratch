import { readURL } from "../../dom/failures";
import { currentPageContext } from "./page_context";

type Link = { url: string; label: string; source?: string };

// The chat's own events (codex.Outgoing's script), which bubble from its form
// or the chat. collect asks for the links a message sends, to push onto
// links; render asks for a sent message's links drawn as chips.
declare global {
  interface GlobalEventHandlersEventMap {
    "codex:collect-outgoing-links": CustomEvent<{ links: Link[]; includeDraft: boolean }>;
    "codex:render-outgoing-links": CustomEvent<{ bubble: HTMLElement; prompt: string; links: unknown }>;
  }
}

function safeLink(value: unknown): Link | undefined {
  if (typeof value !== "object" || value === null) return undefined;
  const { url, label, source } = value as Partial<Link>;
  if (typeof url !== "string" || !url || typeof label !== "string" || !label) return undefined;
  // A malformed link stays plain text.
  const target = readURL(url.toLowerCase().startsWith("www.") ? "https://" + url : url, location.origin);
  if (target && (target.protocol === "http:" || target.protocol === "https:") && !target.username && !target.password) {
    return { url, label, ...(source ? { source } : {}) };
  }
  return undefined;
}

document.addEventListener("codex:collect-outgoing-links", event => {
  const form = event.target;
  if (!(form instanceof HTMLFormElement)) return;
  const detail = event.detail;
  if (detail.includeDraft) {
    form.querySelectorAll<HTMLElement>('[data-slot="chat-composer-link"]').forEach(chip => {
      const link = safeLink({ url: chip.dataset.url, source: "manual", label: chip.dataset.label || chip.textContent });
      if (link) detail.links.push(link);
    });
  }
  const context = currentPageContext(form);
  const link = safeLink(context);
  if (link) detail.links.push(link);
});

document.addEventListener("codex:render-outgoing-links", event => {
  const chat = event.target;
  if (!(chat instanceof HTMLElement)) return;
  const { bubble, prompt, links } = event.detail;
  const template = chat.querySelector<HTMLTemplateElement>('[data-outgoing-template="link"]');
  if (!template || !Array.isArray(links)) return;
  const known = links.map(safeLink).filter((link): link is Link => !!link);
  const target = bubble.querySelector<HTMLElement>('[data-slot="user-message-text"]')!;
  const content = document.createDocumentFragment();
  let offset = 0;
  while (offset < prompt.length) {
    let next: { link: Link; index: number } | undefined;
    for (const link of known) {
      const index = prompt.indexOf(link.url, offset);
      if (
        index >= 0 && (!next || index < next.index || (index === next.index && link.url.length > next.link.url.length))
      ) {
        next = { link, index };
      }
    }
    if (!next) break;
    let before = prompt.slice(offset, next.index);
    if (next.link.source === "automatic" && before.endsWith("\n\nCurrent page: ")) {
      before = before.slice(0, -"Current page: ".length);
    }
    content.append(document.createTextNode(before));
    const chip = template.content.firstElementChild?.cloneNode(true);
    if (!(chip instanceof HTMLElement)) break;
    const anchor = chip.querySelector<HTMLAnchorElement>('[data-slot="link-chip"]')!;
    const destination = new URL(
      next.link.url.toLowerCase().startsWith("www.") ? "https://" + next.link.url : next.link.url,
      location.origin,
    );
    if (destination.protocol !== "http:" && destination.protocol !== "https:") return;
    anchor.href = destination.href;
    anchor.title = next.link.url;
    anchor.target = "_blank";
    anchor.rel = "noopener noreferrer";
    anchor.querySelector("span")!.textContent = next.link.label;
    content.append(chip);
    offset = next.index + next.link.url.length;
  }
  content.append(document.createTextNode(prompt.slice(offset)));
  target.replaceChildren(content);
});
