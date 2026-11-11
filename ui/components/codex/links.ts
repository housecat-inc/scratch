import { setAttributeIfChanged } from "../../dom/dom";
import { registerComponent } from "../../dom/lifecycle";
import { urlAPI } from "../../dom/template_scripts/url";

const chats = new Set<HTMLElement>();

const selector = 'a[data-chat-link="inherited"]';
// The href each link had before updateLink gave it this page's list context,
// with the href updateLink wrote, so a later page recomputes the context from
// the link as rendered rather than from the last page's.
const rendered = new WeakMap<HTMLAnchorElement, { base: string; written: string }>();

// listOf is the list a /mail query shows, by the server's rule
// (HousecatURL.mailListContext), less the chat each link sets for itself; or
// null on the landing view's first page, which a thread or draft is drawn
// over anyway. A page without HousecatURL (a bare test page) shows none.
function listOf(query: string): URLSearchParams | null {
  const rules = urlAPI as Window["HousecatURL"] | undefined;
  const list = rules?.mailListContext(query);
  if (!list) return null;
  list.delete("chat");
  return [...list.keys()].every(key => key === "account") ? null : list;
}

function sameAccount(a: URLSearchParams, b: URLSearchParams): boolean {
  return (a.get("account") || "").trim().toLowerCase() === (b.get("account") || "").trim().toLowerCase();
}

// housecatLabel is a list of a Housecat label, which collects threads from
// every mailbox, so its URL need not name one: a numeric label with provider
// housecat or none, as the server routes it.
function housecatLabel(params: URLSearchParams): boolean {
  const provider = params.get("provider") || "";
  return /^[1-9]\d*$/.test(params.get("label") || "") && (provider === "" || provider === "housecat");
}

// pageList is the list this tab is showing: the list itself, or for a
// thread, the list it was read from, which its back names.
function pageList(page: URL): URLSearchParams | null {
  if (page.pathname === "/mail") return listOf(page.search);
  const back = page.searchParams.get("back") || "";
  return listOf(back.startsWith("/mail?") ? back.slice("/mail".length) : page.search);
}

// PageContext is the page updateLink reads, parsed when a link first asks
// for it at an address and shared, read only, by every link until the
// address changes, so a pass or a mutation with no link to update parses
// nothing.
type PageContext = { href: string; list: URLSearchParams | null; mail: boolean; page: URL };
let parsedPage: PageContext | null = null;

function pageContext(): PageContext {
  const href = location.href;
  if (parsedPage?.href === href) return parsedPage;
  const page = new URL(href);
  const mail = page.pathname === "/mail" || /^\/mail\/[^/]+$/.test(page.pathname);
  parsedPage = { href, list: mail ? pageList(page) : null, mail, page };
  return parsedPage;
}

// inheritListContext keeps the list this tab is showing under a chat link
// that names none, so opening it keeps the sidebar's view: a thread carries
// the list as a row's link does, so Back and Archive return to it and to the
// page of it; a draft carries it so the server opens the draft over it, or
// over its thread for a reply. A link that names its own list, one in
// another mailbox than the list's (a Housecat label's list spans them all),
// or one opened from the landing view's first page is left as written.
function inheritListContext(url: URL, { list, mail, page }: PageContext): void {
  const thread = /^\/mail\/[^/]+$/.test(url.pathname);
  const draft = url.pathname === "/mail" && !!url.searchParams.get("draft");
  if (!(thread || draft) || !mail || !list) return;
  // A bare test page has no HousecatURL.
  const rules = urlAPI as Window["HousecatURL"] | undefined;
  if (url.searchParams.has("back") || rules?.namesMailList(url.searchParams) === true) return;
  if (!sameAccount(url.searchParams, page.searchParams) && !housecatLabel(page.searchParams)) return;
  for (const [key, value] of list) if (key !== "account") url.searchParams.set(key, value);
  if (thread) url.searchParams.set("back", "/mail?" + list.toString());
}

function updateLink(link: HTMLAnchorElement): void {
  const chat = link.closest<HTMLElement>("[data-chat]");
  if (!chat) return;
  const current = link.getAttribute("href");
  if (current === null) return;
  let memo = rendered.get(link);
  if (!memo || memo.written !== current) memo = { base: current, written: current };
  const url = new URL(memo.base, location.href);
  if (url.origin !== location.origin) return;
  inheritListContext(url, pageContext());
  const selected = chat.dataset.selectedThread || "new";
  url.searchParams.set("chat", selected);
  const href = url.pathname + url.search + url.hash;
  rendered.set(link, { base: memo.base, written: href });
  setAttributeIfChanged(link, "href", href);
  if (!link.hasAttribute("hx-get")) return;
  setAttributeIfChanged(link, "hx-push-url", href);
  // _partial leads, where the server's sorted query puts it, so a link the
  // page adds nothing to keeps the hx-get it was rendered with rather than
  // being rewritten, and observed again, on its first pass.
  url.searchParams.delete("_partial");
  setAttributeIfChanged(link, "hx-get", url.pathname + "?_partial=1&" + url.searchParams.toString());
  // htmx reads hx-get and hx-push-url when the link is clicked, so a link it
  // has set up (data-htmx-powered) takes a new value as it is; processing it
  // again would only rescan it. A link htmx has not reached yet is set up here.
  if (!link.hasAttribute("data-htmx-powered")) window.htmx?.process(link);
}

function updateWithin(root: Element): void {
  if (root instanceof HTMLAnchorElement && root.matches(selector)) updateLink(root);
  for (const link of root.querySelectorAll<HTMLAnchorElement>(selector)) updateLink(link);
}
function update(): void {
  for (const chat of chats) if (chat.isConnected) updateWithin(chat);
}

for (const name of ["DOMContentLoaded", "htmx:after:settle", "htmx:after:history:update", "mail:chat-context"]) {
  document.addEventListener(name, update);
}
window.addEventListener("popstate", update);
for (const name of ["pointerdown", "click", "contextmenu", "focusin"]) {
  document.addEventListener(name, event => {
    const link = event.target instanceof Element ? event.target.closest<HTMLAnchorElement>(selector) : null;
    if (link) updateLink(link);
  }, true);
}
registerComponent("[data-chat]", (chat, signal) => {
  chats.add(chat);
  const observer = new MutationObserver(changes => {
    for (const change of changes) {
      if (change.type === "childList") {
        for (const node of change.addedNodes) if (node instanceof Element && node.isConnected) updateWithin(node);
      } else if (change.target === chat && change.attributeName === "data-selected-thread") {
        updateWithin(chat);
      } else if (change.target instanceof HTMLAnchorElement && change.target.matches(selector)) {
        updateLink(change.target);
      }
    }
  });
  observer.observe(chat, {
    attributeFilter: ["data-selected-thread", "data-chat-link", "href", "hx-get"],
    attributes: true,
    childList: true,
    subtree: true,
  });
  signal.addEventListener("abort", () => {
    observer.disconnect();
    chats.delete(chat);
  }, { once: true });
  updateWithin(chat);
});
