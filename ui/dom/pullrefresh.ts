import * as Effect from "effect/Effect";
import * as Fiber from "effect/Fiber";
import { loading, on, partial, redirected, shown } from "./content";
import { morph, type MorphRule } from "./morph";
import { toastAPI } from "./template_scripts/toast";

// Pull to refresh's request and swap run here, on one lane; the gesture, the
// indicator and the spacer stay in MailPullRefreshScript, which calls
// window.HousecatPullRefresh. A refresh is a content intent like a tap: it
// is refused while a content navigation or another refresh is under way,
// and a content navigation that departs ends it and aborts its request, so
// a refresh asked for before a tap never lands over the page the tap
// brought. The list is morphed rather than rebuilt (LIST_MORPH), so the
// scroller and the rows the refresh keeps are the same nodes. An open
// thread's reader refreshes through its own window.mailRefresh, which
// morphs the feed; its request is the reader's, so ending the lane stops
// the wait, not the request. A list request the server redirected to
// another page brought that page, not this one's list, so the lane ends and
// hands it to HousecatContent.reload, which follows the redirect in place of
// the entry shown, as the browser's reload does, and lands the page under
// its own URL.

export type Outcome = "busy" | "done" | "failed" | "interrupted" | "redirected";

// Answer is the list request's answer: the refreshed page's html, or moved
// when the server sent the request on to another page (see moved).
export type Answer = { html: string } | { moved: true };

export type PullRefresh = {
  // Whether a pull may start: no refresh and no content navigation under way.
  idle: () => boolean;
  // Ends the refresh under way and aborts its request.
  interrupt: () => void;
  // Refreshes pane, the mail list or an open thread's reader, and says how
  // it ended; "busy" when the lane refused it.
  refresh: (pane: Element) => Promise<Outcome>;
};

// PullRefreshHost is the page as the lane touches it: browserHost() is the
// document's, and a test passes its own.
export type PullRefreshHost = {
  // Morphs the refreshed page html over the one pane is on.
  apply: (pane: Element, html: string) => Promise<void>;
  // url's answer; rejects for an answer that is not ok, and as fetch does
  // once signal aborts.
  fetch: (url: string, signal: AbortSignal) => Promise<Answer>;
  isReader: (pane: Element) => boolean;
  // The partial request for the page shown.
  listURL: () => string;
  // The pull's share of data-loading, which the content lane writes.
  loading: (on: boolean) => void;
  // Whether a content navigation is in flight.
  navigating: () => boolean;
  // Runs fn as each content navigation departs; returns the removal.
  onDepart: (fn: () => void) => () => void;
  refreshReader: () => Promise<unknown>;
  // Draws the page shown again through the content lane.
  reload: () => void;
  toast: (message: string) => void;
};

export const FAILED = "Could not refresh mail";

// PullRefreshRuntime is the Effect the lane uses, so a test can hand it its
// own. The default names each function rather than passing the modules whole,
// so a bundle keeps only these of Effect.
export type PullRefreshRuntime = {
  Effect: Pick<
    typeof Effect,
    "ensuring" | "gen" | "matchEffect" | "runFork" | "succeed" | "sync" | "tryPromise"
  >;
  Fiber: Pick<typeof Fiber, "interrupt">;
};

const effect: PullRefreshRuntime = {
  Effect: {
    ensuring: Effect.ensuring,
    gen: Effect.gen,
    matchEffect: Effect.matchEffect,
    runFork: Effect.runFork,
    succeed: Effect.succeed,
    sync: Effect.sync,
    tryPromise: Effect.tryPromise,
  },
  Fiber: { interrupt: Fiber.interrupt },
};

// Lane is the fiber of the refresh under way.
type Lane = Fiber.Fiber<Outcome>;

export function createPullRefresh(
  { Effect, Fiber }: PullRefreshRuntime = effect,
  host: PullRefreshHost = browserHost(),
): PullRefresh {
  // current is the lane: the fiber of the refresh under way, cleared by
  // identity once it ends.
  let current: Lane | null = null;

  // request fetches url; interrupting it aborts the signal tryPromise hands
  // the fetch.
  const request = (url: string) => Effect.tryPromise((signal) => host.fetch(url, signal));

  // A refresh that fails leaves the page as it was and says so; one a
  // navigation ends says nothing, since the navigation is what the user sees.
  const program = (pane: Element) =>
    Effect.gen(function*() {
      if (host.isReader(pane)) {
        yield* Effect.tryPromise(() => host.refreshReader());
        return "done" as const;
      }
      const answer = yield* request(host.listURL());
      if ("moved" in answer) return "redirected" as const;
      yield* Effect.tryPromise(() => host.apply(pane, answer.html));
      return "done" as const;
    }).pipe(
      Effect.matchEffect({
        onFailure: () =>
          Effect.sync((): Outcome => {
            host.toast(FAILED);
            return "failed";
          }),
        onSuccess: (outcome) => Effect.succeed<Outcome>(outcome),
      }),
      Effect.ensuring(Effect.sync(() => host.loading(false))),
    );

  function interrupt(): void {
    if (current) Effect.runFork(Fiber.interrupt(current));
  }

  host.onDepart(interrupt);

  return {
    idle: () => current === null && !host.navigating(),
    interrupt,
    refresh(pane) {
      if (current !== null || host.navigating()) return Promise.resolve<Outcome>("busy");
      host.loading(true);
      const fiber = Effect.runFork(program(pane));
      current = fiber;
      // A redirected refresh reloads only once the lane is clear, so the
      // navigation the reload departs finds no refresh to end.
      return new Promise<Outcome>((resolve) => {
        fiber.addObserver((exit) => {
          if (current === fiber) current = null;
          const outcome = exit._tag === "Success" ? exit.value : "interrupted";
          if (outcome === "redirected") host.reload();
          resolve(outcome);
        });
      });
    },
  };
}

// LIST_MORPH_IGNORE are the attributes the page's scripts write inside the
// mail page and the server never renders: an open menu or popover, a star
// whose save is out, the collection's current row and roving tabindex
// (ui/components/collection/collection.ts), a row leaving optimistically,
// and a swipe's state and drag (ui/dom/swipe.ts). A swipe's commit button,
// tray markers and captions are the server's, so they morph.
export const LIST_MORPH_IGNORE = [
  "aria-busy",
  "aria-expanded",
  "data-current",
  "data-done-pending",
  "data-optimistic-busy",
  "data-popup-open",
  "data-swipe",
  "data-swipe-active",
  "data-swipe-dragging",
  "tabindex",
] as const;

// STAR_PENDING are what an optimistic star (ThreadFrameScript) draws while
// its save is out: the state it shows, which the swipe tray's caption
// follows, the swipe tray's star's label for it, and the flip its next tap
// sends. The server's copy may predate the save, so a refresh keeps them
// until the save settles; the star puts them back itself if the save fails.
const STAR_PENDING = ["aria-label", "aria-pressed", "hx-post"] as const;

// SWIPE_VARS are the declarations a swipe draws on a row's style and on its
// tray's parts' (ui/dom/swipe.ts, ui/dom/swipemotion.ts): how far the row has
// slid, and how far each action has grown in. The server renders none.
const SWIPE_VARS = /--swipe-[\w-]+:\s*[^;]+?(?=\s*;|\s*$)/g;

// LIST_MORPH keeps what the page drew on the rows a refresh keeps: the
// attributes above, how far an open swipe row has slid and its actions have
// grown in (SWIPE_VARS), the outline
// of the row the user is on (ui/uikit/mail_navigation.ts), a star whose save
// is out (STAR_PENDING), and a date the page has put in local time
// (MailTimeScript) while the instant it shows is the one the server sent, so
// a row the refresh did not change is not written at all. The row's read
// state, importance and labels are the label store's, which binds a row a
// swap drew again afresh (ThreadFrameScript).
export const LIST_MORPH: MorphRule = {
  carry(from, to) {
    if (from.classList.contains("focused") && from.matches("[data-slot=mail-row]")) to.classList.add("focused");
    if (from.matches("[data-swipe]") || from.matches("[data-swipe-part]")) {
      const drawn = (from.getAttribute("style") ?? "").match(SWIPE_VARS);
      if (drawn) {
        const own = to.getAttribute("style");
        to.setAttribute("style", (own ? [own, ...drawn] : drawn).join(";"));
      }
    }
    if (from.getAttribute("data-optimistic-busy") === "true" && from.matches("[data-optimistic-star]")) {
      for (const name of STAR_PENDING) {
        const value = from.getAttribute(name);
        if (value !== null) to.setAttribute(name, value);
      }
    }
    const ms = from.getAttribute("data-local-time-ms");
    if (ms === null || ms !== to.getAttribute("data-local-time-ms")) return;
    to.innerHTML = from.innerHTML;
    const title = from.getAttribute("title");
    if (title !== null) to.setAttribute("title", title);
  },
  ignore: LIST_MORPH_IGNORE,
};

const LIST = "[data-slot=mail-list]";
const ROW = "[data-slot=mail-row][data-id]";
const SPACER = ":scope > [data-mail-pull-spacer]";

// keepPlace notes where list stands and returns what puts it back once the
// morph is in: at the top it shows from its top, new mail first; scrolled
// down, the first of the rows showing below its head that the refresh kept
// goes back to the offset it had, so a row the refresh took away hands its
// place to the next one. A browser that anchors scrolling has already put
// it there, and then nothing moves.
function keepPlace(list: Element): () => void {
  const spacer = list.querySelector<HTMLElement>(SPACER);
  if (list.scrollTop <= (spacer?.offsetHeight ?? 0) + 1) {
    return () => {
      list.scrollTop = 0;
    };
  }
  const top = list.getBoundingClientRect().top;
  const shown = top + (parseFloat(getComputedStyle(list).paddingTop) || 0);
  const marks: { offset: number; row: HTMLElement }[] = [];
  for (const row of list.querySelectorAll<HTMLElement>(ROW)) {
    const box = row.getBoundingClientRect();
    if (box.bottom > shown) marks.push({ offset: box.top - top, row });
    if (marks.length >= 5) break;
  }
  return () => {
    const kept = marks.find((mark) => list.contains(mark.row));
    if (!kept) return;
    list.scrollTop += kept.row.getBoundingClientRect().top - list.getBoundingClientRect().top - kept.offset;
  };
}

// apply morphs the refreshed page over [data-mail-content], as
// HousecatContent.reload does, so the list header and the list are morphed
// in place and the answer's out-of-band parts -- the sidebar and its counts,
// a toast -- land as a navigation's would. The spacer the pull put at the top
// of the list gets a stand-in with its id at the top of the refreshed list;
// htmx matches the two by id, and the spacer's hx-morph-skip keeps it, and
// the indicator in it, as they are.
function apply(pane: Element, html: string): Promise<void> {
  const content = pane.closest("[data-mail-content]");
  const htmx = window.htmx;
  if (!content || !htmx || !pane.isConnected) return Promise.resolve();
  const page = document.createElement("template");
  page.innerHTML = html;
  const spacer = pane.querySelector(SPACER);
  const fresh = page.content.querySelector(LIST);
  if (spacer?.id && fresh) {
    const stand = document.createElement("div");
    stand.id = spacer.id;
    fresh.prepend(stand);
  }
  const back = keepPlace(pane);
  return morph(htmx, content, page.innerHTML, LIST_MORPH).then(back);
}

// moved is whether res, the answer to the list request asked, was
// redirected to a page other than the one asked for, or lost the partial
// marker on the way and so is a whole document: either way it is not the
// shown page's list, and a morph would draw it under the shown page's URL.
export function moved(res: { redirected: boolean; url: string }, asked: string): boolean {
  const to = redirected(res);
  return to !== null && (!to.partial || to.page !== shown(new URL(asked, res.url)));
}

export function browserHost(): PullRefreshHost {
  return {
    apply,
    fetch: (url, signal) =>
      window.fetch(url, { credentials: "same-origin", signal }).then((r): Answer | Promise<Answer> => {
        if (!r.ok) throw new Error("The list answered " + String(r.status));
        if (moved(r, url)) return { moved: true };
        return r.text().then((html) => ({ html }));
      }),
    isReader: (pane) => pane.id === "readerBody",
    listURL: () => partial(new URL(location.href)),
    loading,
    navigating: () => window.HousecatContent.current() !== null,
    onDepart: (fn) =>
      on("depart", () => {
        fn();
      }),
    refreshReader: () => {
      const run = window.mailRefresh;
      return run ? Promise.resolve().then(run) : Promise.reject(new Error("The reader has no refresh"));
    },
    reload: () => window.HousecatContent.reload(),
    toast: (message) => toastAPI.show({ message }),
  };
}

declare global {
  interface Window {
    HousecatPullRefresh?: PullRefresh;
    mailRefresh?: () => Promise<unknown>;
  }
}

if (typeof window !== "undefined") {
  window.HousecatPullRefresh = createPullRefresh();
}
