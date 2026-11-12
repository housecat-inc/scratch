import { urlAPI } from "./template_scripts/url";
// The content lane owns the one in-flight navigation into
// [data-mail-content]. htmx stays the requester; this module records each
// content request as a Nav when htmx configures it, admits it once htmx has,
// and lets the newest intent win: an earlier GET still in flight is aborted,
// and one whose response has already arrived is vetoed before htmx can push
// its URL or swap it in. GET and POST are both recorded, so loading state
// (data-loading on main is written here alone) and the arrive hooks cover the
// settings card's posts and their redirect landings; only GETs are aborted
// and vetoed, because a request the server has received must land.
//
// The chat rule (decide) is pure and lives here so Vitest runs it in Node:
// nothing in this file reaches the document outside install(), and
// window.HousecatViewport is read at call time, not at import.
//
// It is also the one listener for a history restore: it keeps the page main
// shows and decides, for every traversal, whether that page is already there
// or is swapped in.
//
// visit and reload are here too, so one copy decides what swaps: a visit is
// a request into the content div like any link's, so the lane records it,
// and a redirect it meets is followed to the page the redirect names.

import * as Wire from "../schemas/schemas.gen";
import { scrubPath } from "./clienterror";
import { readJSON } from "./failures";
import { readDecoded } from "./wire";

export type HtmxCtx = {
  // Set through htmx.ajax's options by ContentNavigationScript for the
  // settings card's posts, which follow their own redirects.
  cardPost?: boolean;
  // Set by mail_sidebar.ts as a request goes out: the sidebar sections
  // saved then (shellprefs.ts's navSections), a copy of its own.
  navSections?: Record<string, boolean>;
  select?: string;
  sourceEvent?: Event;
  // Set through htmx.ajax's options by visit and reload: the hash their URL
  // named, which stays out of the request and rides on to the page a
  // redirect lands on, as a browser carries it across one.
  hash?: string;
  // The response's HX-* headers, read before htmx follows HX-Redirect.
  hx?: { redirect?: string };
  // What htmx pushes once it swaps; false for no entry. Read inside swap(),
  // so a handler at htmx:after:request may still set it.
  push?: string | boolean | null;
  // Set through htmx.ajax's options by reload, and by the landing a
  // reload's redirect leads to: the page landed on takes the place of the
  // entry reloaded, as the browser's own reload puts it.
  reload?: boolean;
  // What htmx puts in place of the current entry once it swaps; read where
  // push is.
  replace?: string | boolean | null;
  // body is the request's values at htmx:config:request: a GET's are folded
  // into its URL by htmx only after that event.
  request: {
    abort: () => void;
    action: string;
    body?: FormData | null;
    method: string;
    signal: AbortSignal;
    submitter?: Element | null;
  };
  response?: { headers?: Headers; raw: Response; status: number };
  // Set through htmx.ajax's options by the restore listener below, the way
  // cardPost travels.
  returning?: boolean;
  sourceElement: Element | null;
  // Where htmx is in the request: "issuing", "swapped", "error: ...".
  status?: string;
  swap?: string;
  target: Element | null;
  // The response's body, once htmx has read it.
  text?: string;
  // Set through htmx.ajax's options by visit and reload, whose request
  // follows a redirect to the page it names.
  visit?: boolean;
};

export type HtmxDetail = { ctx?: HtmxCtx; tasks?: { fragment?: DocumentFragment; target?: Element }[] } | null;

// The htmx events the kit listens for, on the document, as it reads their
// detail. htmx's own typings (htmx.org/dist/htmx.d.ts) leave out fields htmx
// sets, such as status and the response's hx headers, and say that fields
// that can be null, such as target and push, can't be, so the kit declares
// its own. A script or a browser test may fire one bare, with no detail.
declare global {
  interface DocumentEventMap {
    "htmx:after:request": CustomEvent<HtmxDetail>;
    "htmx:after:settle": CustomEvent<HtmxDetail>;
    "htmx:after:swap": CustomEvent<HtmxDetail>;
    "htmx:before:history:restore": CustomEvent<{ path?: string } | null>;
    "htmx:before:history:update": CustomEvent<
      { response?: unknown; history?: { path?: string }; sourceElement?: Element } | null
    >;
    "htmx:before:request": CustomEvent<HtmxDetail>;
    "htmx:before:response": CustomEvent<HtmxDetail>;
    "htmx:before:swap": CustomEvent<HtmxDetail>;
    "htmx:config:request": CustomEvent<HtmxDetail>;
    "htmx:error": CustomEvent<HtmxDetail>;
    "htmx:finally:request": CustomEvent<HtmxDetail>;
  }
}

// Issued is what the chat rule needs to know about the moment a request was
// made: which chat the URL named, whether the viewport was compact, and
// whether the source sat inside the chat rail.
export type Issued = { chat: string | null; compact: boolean; railSource: boolean };

export type Nav = {
  ctx: HtmxCtx;
  issued: Issued;
  // The request verb, upper case. Everything but GET lands.
  method: string;
  // A history restore, or the reader's Back button.
  returning: boolean;
  // The page the request lands on, without _partial: the request URL, or,
  // once a visit the server redirected is admitted, the page the redirect
  // names.
  url: URL;
};

export type Decision = "as-is" | "carry-chat" | "close-chat" | "drop-chat";

// A mail page is the list or one thread; the chat path and the card pages are
// not, and neither is anything deeper under a thread.
const MAIL_PAGE = /^\/mail(?:\/[^/?#]+)?$/;

// decide is the chat rule for a history entry htmx is about to write while
// the URL names a chat, read from the snapshot of the request whose swap is
// running rather than from the live URL, which a later open may have
// changed. Docked, and for a request issued before chat opened, the page
// lands under the chat view and the view stays up (carry). Compact, a
// request from outside the rail while chat showed is the user asking for
// mail (drop). Compact, a link inside the rail to a mail page reveals Mail
// (close); to any other page it is left as it is.
export function decide(nav: Nav | null, entryPath: string): Decision {
  if (!nav) return "as-is";
  const { chat, compact, railSource } = nav.issued;
  if (!compact || chat === null) return "carry-chat";
  if (!railSource) return "drop-chat";
  return MAIL_PAGE.test(entryPath.split(/[?#]/, 1)[0] ?? "") ? "close-chat" : "as-is";
}

// A depart hook may answer false to veto a request while htmx can still
// drop it; an arrive hook runs when the current navigation ends, swapped or
// not; a restore hook runs as a history traversal's page is swapped in,
// before its request exists as a Nav, with the page being brought back. A
// Set keeps a hook registered once however often it is passed.
export type Hook = (nav: Nav) => boolean | void;
export type Restore = { url: URL };
export type RestoreHook = (restore: Restore) => void;
export type HookName = "arrive" | "depart" | "restore";

export const hooks = { arrive: new Set<Hook>(), depart: new Set<Hook>(), restore: new Set<RestoreHook>() };

export function on(hook: "arrive" | "depart", fn: Hook): () => void;
export function on(hook: "restore", fn: RestoreHook): () => void;
export function on(hook: HookName, fn: Hook | RestoreHook): () => void {
  const set = hooks[hook] as Set<Hook | RestoreHook>;
  set.add(fn);
  return () => {
    set.delete(fn);
  };
}

// pageOf is the page a URL names for the shell's main: its path and query
// without the chat param, which names the chat view over the page, not the
// page, and without the partial-response marker or hash.
export function pageOf(href: string, base: string): string {
  const url = new URL(href, base);
  url.searchParams.delete("chat");
  url.searchParams.delete("_partial");
  return url.pathname + url.search;
}

export type HousecatContentGlobal = {
  current: () => Nav | null;
  entryFor: typeof entryFor;
  landing: () => Nav | null;
  loading: (on: boolean) => void;
  on: typeof on;
  partial: typeof partial;
  policy: { decide: typeof decide };
  reload: typeof reload;
  shown: typeof shown;
  swappable: typeof swappable;
  visit: typeof visit;
};

// htmx's global, as far as the bundle calls it: ajax assigns its options
// onto the request's ctx, which is how returning reaches snapshot(), process
// binds the hx- attributes a script has written onto an element, and
// registerExtension takes ui/dom/morph.ts's hooks.
type HtmxGlobal = {
  ajax: (method: string, url: string, options: Record<string, unknown>) => Promise<unknown>;
  trigger: (element: Element, name: string) => void;
  process: (element: Element) => void;
  registerExtension: (name: string, extension: object) => void;
  swap: (options: { sourceElement: Element; target: Element; swap: string; text: string }) => Promise<void>;
};

declare global {
  interface Window {
    HousecatContent: HousecatContentGlobal;
    // samePath (app/ui/layouts/url.templ) reduces a target the page did not
    // write to a path on this origin; dropToastParams deletes the params that
    // raise a toast once; mailListContext, mailView and namesMailList apply
    // the server's mail URL rules (app/ui/layouts/mail_url.go).
    HousecatURL: {
      externalURL: (raw: string) => string;
      dropToastParams: (params: URLSearchParams) => void;
      mailListContext: (raw: string | URLSearchParams) => URLSearchParams;
      mailView: (raw: string | URLSearchParams) => URLSearchParams;
      namesMailList: (params: URLSearchParams) => boolean;
      samePath: (raw: string) => string;
    };
    htmx?: HtmxGlobal;
  }
}

let current: Nav | null = null;
let landing: Nav | null = null;
const navs = new WeakMap<HtmxCtx, Nav>();
// The requests issued from inside the content, noted as each is issued: a
// swap can take a request's source out of the document before its answer
// comes.
const fromContent = new WeakSet<HtmxCtx>();

// navOf is the Nav the lane recorded for a request's ctx, or null for a swap
// the lane did not make (pull to refresh's htmx.swap). It stays readable
// after the request ends, so a listener at htmx:after:swap can ask.
export function navOf(ctx: HtmxCtx): Nav | null {
  return navs.get(ctx) ?? null;
}
let refreshing = false;

function ctxOf(e: CustomEvent<HtmxDetail>): HtmxCtx | undefined {
  return e.detail?.ctx;
}

function content(ctx: HtmxCtx): boolean {
  return ctx.target instanceof Element && ctx.target.matches("[data-mail-content]");
}

function main(): Element | null {
  return document.querySelector("[data-slot=app-main]");
}

function paintLoading(): void {
  main()?.toggleAttribute("data-loading", current !== null || refreshing);
}

// Pull to refresh (ui/dom/pullrefresh.ts) runs outside htmx. Its refresh
// contributes to the same indicator without clearing a content request that
// is still in flight.
export function loading(on: boolean): void {
  refreshing = on;
  paintLoading();
}

// The card's pages, by path prefix, as MailNavigationScript renders them
// (settings-card-paths, a ui/wire SettingsCardPaths); read once, on first use,
// since the script is in the page before this bundle runs and no swap brings
// another copy.
let cardPathList: ReadonlyArray<string> | null = null;

function cardPaths(): ReadonlyArray<string> {
  if (cardPathList) return cardPathList;
  const paths = readJSON(document.getElementById("settings-card-paths")?.textContent);
  cardPathList = paths === undefined
    ? []
    : readDecoded(
      Wire.SettingsCardPaths,
      { endpoint: scrubPath(location.pathname), operation: "content.settings-card-paths" },
      paths,
    ) ?? [];
  return cardPathList;
}

// swappableURL is the URL path names when its page can be swapped into the
// content div: a mail page or a card page on base's origin, not a download.
// base and the card paths are arguments so the rule runs in Node; swappable
// below reads them from the document. Its pathname starts with one slash and
// a mail or card prefix, so it can never be read as a host.
export function swappableURL(path: string, base: string, cardPaths: ReadonlyArray<string>): URL | null {
  let url: URL;
  try {
    url = new URL(path, base);
  } catch {
    return null;
  }
  if (url.origin !== new URL(base).origin || url.searchParams.get("download") === "1") return null;
  if (/^\/mail(?:\/|$)/.test(url.pathname)) return url;
  return cardPaths.some((p) => url.pathname === p || url.pathname.startsWith(p + "/")) ? url : null;
}

export function swappable(path: string): URL | null {
  return swappableURL(path, location.href, cardPaths());
}

// partial is the request a swap makes for url: the page's content alone,
// without the hash, which the server never sees.
export function partial(url: URL): string {
  const get = new URL(url.href);
  get.searchParams.set("_partial", "1");
  get.hash = "";
  return get.pathname + get.search;
}

// shown is url as the address bar shows it: without the partial marker, hash
// kept.
export function shown(url: URL): string {
  const u = new URL(url.href);
  u.searchParams.delete("_partial");
  return u.pathname + u.search + u.hash;
}

export type Redirected = { page: string; partial: boolean };

// redirected is where a request ended when the server redirected it: the
// page the final URL names, and whether that URL still asks for the partial,
// so the body is the page's content, or lost the marker, so the body is a
// whole document.
export function redirected(res: { redirected: boolean; url: string }): Redirected | null {
  if (!res.redirected) return null;
  const url = new URL(res.url);
  return { page: shown(url), partial: url.searchParams.get("_partial") === "1" };
}

// pushFor is the history entry a swap of the page to makes from the page
// here: none when it is the page already shown, as a browser adds none for
// a link to the page it is on.
export function pushFor(to: string, here: string): string | false {
  return to === here ? false : to;
}

function inShell(): boolean {
  return !!window.htmx && !!document.querySelector("[data-mail-content]");
}

// here is the address bar's page, in the form shown and pushFor compare.
function here(): string {
  return location.pathname + location.search + location.hash;
}

// entryFor is the history entry a swap of url's page makes from the page
// shown now.
export function entryFor(url: URL): string | false {
  return pushFor(shown(url), here());
}

// visit swaps href's page into the content div as a link would and pushes
// its URL, wherever the shell's content div is in the document; outside the
// shell, or for a page that cannot be swapped, the browser navigates. The
// request goes through the lane like any other: recorded, superseding an
// earlier GET, drawing the loading bar. A hash rides to the pushed URL and
// stays out of the GET.
export function visit(href: string): void {
  land(href, false);
}

// land is visit, or, with reloading, the landing of a reload the server
// redirected: that page takes the place of the entry reloaded rather than
// being pushed after it, as the browser's own reload puts it.
function land(href: string, reloading: boolean): void {
  const url = swappable(href);
  if (!url || !inShell()) {
    const to = urlAPI.sameOriginURL(href)?.href ?? location.href;
    if (reloading) location.replace(to);
    else location.href = to;
    return;
  }
  const entry = entryFor(url);
  void window.htmx!.ajax("GET", partial(url), {
    hash: url.hash,
    push: reloading ? false : entry,
    reload: reloading,
    replace: reloading ? entry : false,
    swap: "innerHTML",
    target: "[data-mail-content]",
    visit: true,
  });
}

// reload draws the page main shows again over itself, morphed so its scroll
// and focus stay, for a page that changed something with htmx and needs to
// show it; outside the shell the browser reloads. A page that now redirects
// is followed to the page it names, as the browser's reload would be: in
// place of the entry reloaded, and drawn fresh, since it is another page.
export function reload(): void {
  const url = swappable(location.href);
  if (!url || !inShell()) {
    location.reload();
    return;
  }
  void window.htmx!.ajax("GET", partial(url), {
    hash: url.hash,
    reload: true,
    swap: "innerMorph",
    target: "[data-mail-content]",
    visit: true,
  });
}

function snapshot(ctx: HtmxCtx): Nav {
  const source = ctx.sourceElement;
  const method = ctx.request.method.toUpperCase();
  const url = new URL(ctx.request.action, location.href);
  // A GET's values travel in its query: the Nav names the URL the request
  // is made to, folded the way htmx folds it.
  const body = ctx.request.body;
  if (method === "GET" && body instanceof FormData) {
    for (const key of body.keys()) url.searchParams.delete(key);
    // A file goes in the query as its name, as a browser's own GET submits it.
    for (const [key, value] of body) url.searchParams.append(key, typeof value === "string" ? value : value.name);
  }
  url.searchParams.delete("_partial");
  return {
    ctx,
    issued: {
      chat: new URL(location.href).searchParams.get("chat"),
      compact: window.matchMedia(window.HousecatViewport.compact).matches,
      railSource: !!source?.closest("[data-chat-rail]"),
    },
    method,
    returning: ctx.returning === true || source?.matches("[data-reader-action=back]") === true,
    url,
  };
}

function finish(nav: Nav): void {
  if (nav !== current) return;
  current = null;
  paintLoading();
  hooks.arrive.forEach((fn) => fn(nav));
}

function stale(nav: Nav): boolean {
  return nav.method === "GET" && (nav !== current || nav.ctx.request.signal.aborted);
}

function install(): void {
  let shownPage = pageOf(location.href, location.href);
  // A same-page restore can be cancelled without issuing another request.
  // Capture runs before the shell's restore interceptors can start a new GET.
  document.addEventListener("htmx:before:history:restore", () => {
    const nav = current;
    if (nav?.method !== "GET") return;
    nav.ctx.request.abort();
    if (landing === nav) landing = null;
    finish(nav);
  }, true);

  // The depart veto runs while htmx can still drop the request.
  document.addEventListener("htmx:config:request", (e) => {
    const ctx = ctxOf(e);
    if (!ctx || !content(ctx)) return;
    const nav = snapshot(ctx);
    for (const fn of hooks.depart) {
      if (fn(nav) === false) {
        e.preventDefault();
        return;
      }
    }
    navs.set(ctx, nav);
  });

  // A request is recorded only once htmx has admitted it: one its queue
  // drops never reaches finally, so a record made earlier would stick.
  document.addEventListener("htmx:before:request", (e) => {
    const ctx = ctxOf(e);
    if (ctx?.sourceElement?.closest("[data-mail-content]")) fromContent.add(ctx);
    const nav = ctx && navs.get(ctx);
    if (!nav) return;
    // hx-sync replaces requests sharing a source queue. Content requests
    // also cross queues, and a POST must keep running when a GET starts.
    if (current?.method === "GET") current.ctx.request.abort();
    current = nav;
    paintLoading();
  });

  document.addEventListener("htmx:before:response", (e) => {
    const ctx = ctxOf(e);
    const nav = ctx && navs.get(ctx);
    if (nav && stale(nav)) e.preventDefault();
  });

  // The body can finish after another navigation starts. htmx commits
  // history inside swap(), so admission must precede that call. A visit the
  // server redirected is admitted under the page the redirect names: a
  // redirect that kept the partial marker brought that page's content, which
  // is swapped in and pushed under the named URL; one that dropped the marker
  // brought a whole document, and one to a page the lane cannot swap brought
  // a page that is not the content's, so either is set aside while the named
  // page is visited in its place, which loads in full when it cannot swap. A
  // visit admitted under the named page carries it as its Nav's url, so the
  // arrive hooks see where the visit landed rather than the URL it asked for.
  // A final URL never carries a fragment, so the hash the visit asked for
  // rides on to the page it lands on. A reload's landing replaces the entry
  // reloaded, and is drawn fresh when it is another page.
  document.addEventListener("htmx:after:request", (e) => {
    const ctx = ctxOf(e);
    const nav = ctx && navs.get(ctx);
    if (!nav) return;
    if (stale(nav)) {
      e.preventDefault();
      return;
    }
    const raw = nav.ctx.response?.raw;
    const to = nav.ctx.visit === true && raw ? redirected(raw) : null;
    if (to) {
      const page = to.page + (nav.ctx.hash ?? "");
      const reloading = nav.ctx.reload === true;
      if (!to.partial || !swappable(to.page)) {
        e.preventDefault();
        land(page, reloading);
        return;
      }
      const entry = pushFor(page, here());
      if (reloading) {
        nav.ctx.replace = entry;
        if (entry !== false) nav.ctx.swap = "innerHTML";
      } else {
        nav.ctx.push = entry;
      }
      nav.url = new URL(to.page, location.href);
    }
    landing = nav;
  });

  // An htmx request made from inside the content whose answer names a page
  // with HX-Redirect -- the reader's Unarchive -- lands there by visit, as a
  // link would, rather than by htmx's own location.href, which would load
  // the shell again; that holds for one whose source a swap has since taken
  // out of the document, so where the request came from is read as it was
  // issued. A page the lane cannot swap is left to htmx. The settings card's
  // posts follow theirs in ContentNavigationScript.
  document.addEventListener("htmx:after:request", (e) => {
    const ctx = ctxOf(e);
    const hx = ctx?.hx;
    if (e.defaultPrevented || !ctx || !hx?.redirect || ctx.cardPost === true) return;
    const to = hx.redirect;
    if (!fromContent.has(ctx) || !swappable(to)) return;
    hx.redirect = "";
    ctx.swap = "none";
    visit(to);
  });

  // A later response listener can start a navigation synchronously. Both
  // history and the DOM must veto that superseded response independently.
  document.addEventListener("htmx:before:history:update", (e) => {
    if (landing && e.detail?.response === landing.ctx.response && stale(landing)) e.preventDefault();
  }, true);

  document.addEventListener("htmx:before:swap", (e) => {
    const ctx = ctxOf(e);
    const nav = ctx && navs.get(ctx);
    if (nav && stale(nav)) e.preventDefault();
  });

  document.addEventListener("htmx:after:swap", (e) => {
    const ctx = ctxOf(e);
    const nav = ctx && navs.get(ctx);
    if (ctx && nav && content(ctx) && !/^none(?:\s|$)/.test(ctx.swap ?? "")) {
      const href = nav.returning ? ctx.response?.raw.url || nav.url.href : location.href;
      shownPage = pageOf(href, location.href);
    }
    if (nav === landing) landing = null;
  });

  // A visit stands in for a full page load, so one whose request gets no
  // response at all -- offline, or a redirect to another origin such as an
  // expired session's sign-in, which a same-origin fetch refuses -- is made
  // as the full load it replaced, and the browser shows what it reaches. A
  // request a newer navigation superseded is aborted, not failed, and stays
  // put.
  document.addEventListener("htmx:error", (e) => {
    const ctx = ctxOf(e);
    const nav = ctx && navs.get(ctx);
    if (ctx?.visit !== true || !nav || ctx.response || ctx.request.signal.aborted) return;
    const to = urlAPI.samePath(nav.url.href + (ctx.hash ?? ""));
    if (ctx.reload === true) location.replace(to);
    else location.href = to;
  });

  document.addEventListener("htmx:finally:request", (e) => {
    const ctx = ctxOf(e);
    const nav = ctx && navs.get(ctx);
    if (!nav) return;
    if (nav === landing) landing = null;
    finish(nav);
  });

  // Only a completed content swap advances shownPage. A pending restore
  // can fail or be vetoed while the browser URL already names its destination.
  // Restoring a page already displayed only repaints the chat view.
  document.addEventListener("htmx:before:history:restore", (e) => {
    if (e.defaultPrevented) return;
    const path = e.detail?.path ?? location.href;
    const page = pageOf(path, location.href);
    if (page === shownPage) {
      e.preventDefault();
      return;
    }
    const url = swappable(path);
    if (!url || !inShell()) return;
    e.preventDefault();
    const restore: Restore = { url };
    hooks.restore.forEach((fn) => fn(restore));
    void window.htmx!.ajax("GET", partial(url), { returning: true, swap: "innerHTML", target: "[data-mail-content]" });
  });

  // A page the browser brings back from its back-forward cache comes back
  // as it was left, a navigation in flight included: the record is dropped
  // and data-loading with it, so the bar is not left running for a request
  // the document's last run made. A response that still arrives is a stale
  // GET and is vetoed above; a POST lands, as every POST does.
  window.addEventListener("pageshow", (e) => {
    if (!e.persisted) return;
    current = null;
    landing = null;
    loading(false);
  });

  window.HousecatContent = {
    current: () => current,
    entryFor,
    landing: () => landing,
    loading,
    on,
    partial,
    policy: { decide },
    reload,
    shown,
    swappable,
    visit,
  };
}

if (typeof document !== "undefined") install();
