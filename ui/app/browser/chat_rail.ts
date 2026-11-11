export let chatRailAPI: typeof housecatChatRail = window.housecatChatRail;
import { keys as keyRouter } from "../../dom/keys";
function isThreadEvent(value: unknown): value is { threadID: string } {
  return (
    value !== null
    && typeof value === "object"
    && "threadID" in value
    && typeof value.threadID === "string"
  );
}
import * as Arr from "effect/Array";
import * as Option from "effect/Option";
import * as Str from "effect/String";
import { docked, isOpen, setDocked, setOpen } from "../../components/drawer/records";
import type { OpenOptions } from "../../dom/chatpanel";
import { required } from "../../dom/required";
import { saveChatLast } from "../../dom/shellprefs";
export function chat_rail(hook: HTMLElement, _swapped: boolean): void {
  (function() {
    // Below the docked width the open rail is the view the stage shows in
    // place of mail, covering the top bar as well as main: keys stay in it
    // and the bottom bar, whose switch is how the user leaves it (search
    // is back on Mail), and Escape closes it. The layer is modal: the key
    // router holds what the view covers -- main, the top bar, the closed
    // sidebar beside them -- inert, and the bar is a region so its switch
    // stays live. The media is the exact
    // complement of the docked breakpoint, as the drawer's max-docked
    // styles and the tour's docked query read it, so no width shows the
    // drawer without its layer or runs the tour beside it. An open auto
    // popover in the rail takes Escape first, through the router's Escape
    // order, and an input method's Escape never reaches the layer. The
    // layer registers on every run, above the guard, so a script the page
    // renders again owns it again; the guard keeps the rail's other
    // listeners from being added twice. On a coarse pointer, Android's back
    // gesture closes it too.
    keyRouter.layer({
      owner: hook,
      back: true,
      escape() {
        return !!chatRailAPI && chatRailAPI.close();
      },
      id: "chat-rail",
      match: "[data-chat-rail][data-drawer-open]",
      media: window.HousecatViewport.compact,
      modal: true,
      regions(el) {
        return Arr.filter(
          [el ?? null, document.querySelector<HTMLElement>("[data-slot=bottombar]")],
          Boolean,
        );
      },
    });
    if (window.__hcChatRailBound === true) return;
    window.__hcChatRailBound = true;
    const CHAT = hook.dataset.chatParam ?? "";
    const CHAT_PATH = hook.dataset.chatPath ?? "";
    const CONTENT = hook.dataset.contentId ?? "";
    const LAST = "hcChatRailLast";
    function rail() {
      return document.querySelector<HTMLElement>("[data-chat-rail]");
    }
    function remember(id: string | null) {
      if (!id) return;
      try {
        window.localStorage.setItem(LAST, id);
      } catch {}
      saveChatLast(id);
    }
    function lastChat() {
      try {
        return window.localStorage.getItem(LAST) || "new";
      } catch {
        return "new";
      }
    }
    // badgeChat is the chat the toggle's badge describes: the one open in
    // the rail while it is open, and while it is closed lastChat(), the
    // same value a click reopens, so a closed badge can never describe a
    // chat other than the one a click brings back. A new chat has no
    // status, so it is none.
    function badgeChat() {
      const id = new URL(location.href).searchParams.get(CHAT) || lastChat();
      return id === "new" ? "" : id;
    }
    // chatStatus prefers the chat mounted in the rail, open or closed, whose
    // header its SSE stream keeps current, over its row in the Chats
    // section, which is absent once the chat falls out of the first five.
    function chatStatus(id: string) {
      if (!id) return "";
      const key = CSS.escape(id);
      const el = rail();
      const header = el?.querySelector<HTMLElement>(
        '[data-chat][data-selected-thread="' + key + '"] [data-chat-status]',
      );
      if (header) return header.getAttribute("data-chat-status") || "";
      const row = document.querySelector<HTMLElement>(
        '[data-recent-chat][data-chat-id="' + key + '"]',
      );
      return row ? row.getAttribute("data-chat-status") || "" : "";
    }
    function paintBadge() {
      const status = chatStatus(badgeChat());
      document.querySelectorAll<HTMLElement>("[data-chat-rail-toggle-status]").forEach((b) => {
        if (b.getAttribute("data-thread-status") !== status) {
          b.setAttribute("data-thread-status", status);
        }
      });
    }
    let badgeQueued = false;
    function queueBadge() {
      if (badgeQueued) return;
      badgeQueued = true;
      queueMicrotask(() => {
        badgeQueued = false;
        paintBadge();
      });
    }
    // The chat row is never the page here: the mailbox the rail rides on
    // always is, so an open chat's row only gets "true" -- selected in its
    // own list, not what fills the main pane.
    function markRecent(id: string | null) {
      document.querySelectorAll<HTMLElement>("[data-recent-chat]").forEach((el) => {
        if (el.getAttribute("data-chat-id") === id) el.setAttribute("aria-current", "true");
        else el.removeAttribute("aria-current");
      });
    }
    function paint(value: boolean | string | null) {
      const open = Boolean(value);
      const el = rail();
      if (!el) return;
      // The URL opens the rail at every width, so it writes both of the
      // drawer's records the same way: open below docked and expanded from
      // docked up, or closed and collapsed out of sight (chatRailDocked).
      setOpen(el, open);
      setDocked(el, open ? "expanded" : "collapsed");
      document.querySelectorAll<HTMLElement>("[data-chat-rail-toggle]").forEach((t) => {
        t.setAttribute("aria-expanded", open ? "true" : "false");
      });
      paintView(open);
      paintBadge();
      paintToggleInset();
      document.dispatchEvent(new CustomEvent("mail:chat-context"));
    }
    // paintView is the phone half of the open state: the bar's switch
    // shows which of the stage's two layers is up, read at use from the
    // records (HousecatShellSwitch.index, the rule the server's first
    // paint shares): chat in the URL, else the card present means
    // neither, else Mail. What the view covers is held inert by the key
    // router, from the layer above, which is confined to compact
    // widths; nothing here writes inert. It runs again when the
    // viewport crosses the breakpoint, since the URL does not change
    // then, and after content swaps, since the card may have
    // come or gone.
    const compactQuery = window.matchMedia(window.HousecatViewport.compact);
    function settingsCard() {
      return document.querySelector<HTMLElement>("[data-mail-content] [data-slot=settings]");
    }
    function paintView(open: boolean) {
      const sw = document.querySelector<HTMLElement>("#mail-shell-switch");
      if (!sw || !("HousecatSegmented" in window) || !("HousecatShellSwitch" in window)) return;
      window.HousecatSegmented.select(
        sw,
        window.HousecatShellSwitch.index({ card: !!settingsCard(), chat: open, rail: !!rail() }),
      );
    }
    compactQuery.addEventListener("change", paintViewNow);
    // --chat-toggle-inset is how far the edge toggle reaches into the mail
    // pane from the pane's right edge; the tray's bottom rows end short of
    // it (--chat-toggle-clearance in app.css). The toggle's width follows its
    // label, the rail's open state and the breakpoint, so it is measured
    // rather than kept in a token. Below docked the toggle is not drawn,
    // so it takes nothing and the inset is zero. It is the button's box,
    // not its surface, so the hover slide never resizes the rows.
    let insetObserver: ResizeObserver | null = null;
    let insetObserved: HTMLElement[] = [];
    function paintToggleInset() {
      const el = rail();
      const toggle = el?.querySelector<HTMLElement>("[data-chat-rail-toggle]");
      const main = document.querySelector<HTMLElement>("[data-slot=app-main]");
      let inset = 0;
      if (toggle && main) {
        const t = toggle.getBoundingClientRect();
        const m = main.getBoundingClientRect();
        if (Math.abs(t.right - m.right) <= 1) inset = Math.max(0, m.right - t.left);
      }
      document.documentElement.style.setProperty("--chat-toggle-inset", inset + "px");
      watchToggleInset([toggle ?? null, main]);
    }
    // A ResizeObserver on the toggle and the pane catches every width change
    // that moves the inset: the label, the open state, the breakpoint, fonts
    // arriving. A node swapped out of the page is let go.
    function watchToggleInset(nodes: (HTMLElement | null)[]) {
      if (typeof ResizeObserver === "undefined") return;
      insetObserver ??= new ResizeObserver(paintToggleInset);
      insetObserved = Arr.filter(insetObserved, (n) => {
        if (n.isConnected) return true;
        required(insetObserver).unobserve(n);
        return false;
      });
      Arr.forEach(nodes, (n) => {
        if (!n || insetObserved.includes(n)) return;
        required(insetObserver).observe(n);
        insetObserved.push(n);
      });
    }
    const CHAT_DEPTH = "hcChatDepth";
    function chatDepth() {
      const state: unknown = history.state;
      const depth = state !== null && typeof state === "object" && CHAT_DEPTH in state
        ? state[CHAT_DEPTH]
        : undefined;
      return typeof depth === "number" ? Math.trunc(depth) : 0;
    }
    function chatState(depth: number) {
      const s: Record<string, unknown> = {};
      if (depth > 0) s[CHAT_DEPTH] = depth;
      return s;
    }
    // HousecatHistory (ui.js, deferred) writes every Housecat entry
    // as one htmx restores; a write that lands before the bundle is
    // in makes the same entry by hand, mark included.
    function pushEntry(patch: Record<string, unknown>, url: string | URL) {
      if ("HousecatHistory" in window) window.HousecatHistory.push(patch, url);
      else history.pushState({ htmx: true, ...patch }, "", url);
    }
    function replaceEntry(patch: Record<string, unknown>, url: string | URL) {
      if ("HousecatHistory" in window) window.HousecatHistory.replace(patch, url);
      else history.replaceState({ ...history.state, ...patch, htmx: true }, "", url);
    }
    // setChatParam also hands a tour the chat it is on, so a tour saved at a
    // step about this chat reopens it after a reload or in another browser.
    // A new chat has no thread yet; its id arrives with the thread.
    //
    // A push that opens chat over a closed page records in its entry's
    // state how many entries back the closed page is (CHAT_DEPTH): 1 for
    // the open itself, one more for each chat pushed after it, a new
    // chat's thread arriving or another chat picked. Closing below docked
    // goes back that far rather than adding a second copy of the page on
    // top. A replace keeps the depth of the entry it rewrites. An entry
    // without one -- the page load, htmx's own -- has nothing to go back
    // to. The depth lives in history.state, not in the URL, which stays
    // the only record of whether chat is open.
    function setChatParam(id: string | null, push: boolean) {
      const url = new URL(location.href);
      let depth = chatDepth();
      if (push) depth = url.searchParams.has(CHAT) ? depth && depth + 1 : 1;
      url.searchParams.set(CHAT, required(id));
      (push ? pushEntry : replaceEntry)(chatState(depth), url);
      remember(id);
      markRecent(id);
      paint(true);
      if (id && id !== "new") {
        document.dispatchEvent(new CustomEvent("tour:state", { detail: { chat: id } }));
      }
    }
    // openChat's options are what the user asked for with this open: draft
    // puts text in the chat's composer, below any text the user already
    // has there, and focus puts the cursor there. Both wait for the
    // composer through withPanel, which calls ready once the chat's own
    // saved draft is back, and the draft is filled wherever the chat
    // opens, while focus is held back on a touch screen or below the
    // docked width, where it raises the on-screen keyboard over the chat.
    // The gate asks for "not coarse" rather than "fine" because a
    // keyboard-only device, headless Chrome among them, reports no pointer
    // at all. Focus also yields when it has moved on from where the open
    // left it, except for a draft: a draft comes from a palette or command
    // bar whose own close hands focus back to the page, and the draft is
    // what the user asked to write next. An open the URL has since left
    // is done: the rail has moved to another chat or closed. fresh is an
    // explicit request to start a new chat: it replaces a new chat whose
    // first message is under way, which any other open of "new" reopens.
    function openChat(
      id: string,
      push: boolean,
      options?: { draft?: string; focus?: boolean; fresh?: boolean },
    ) {
      options ??= {};
      setChatParam(id, push);
      const from = document.activeElement;
      let filled = false;
      withPanel(id, { fresh: options.fresh === true }, (field) => {
        if (new URL(location.href).searchParams.get(CHAT) !== id) return true;
        if (options.draft && !filled) {
          field.value = Str.trim(field.value) ? field.value + "\n" + options.draft : options.draft;
          field.dispatchEvent(new Event("input", { bubbles: true }));
          filled = true;
        }
        if (
          !(options.focus === true)
          || window.matchMedia("(pointer: coarse), " + window.HousecatViewport.compact).matches
        ) {
          return true;
        }
        const active = document.activeElement;
        if (!options.draft && active && active !== document.body && active !== from) return true;
        if (!field.checkVisibility({ visibilityProperty: true })) return false;
        field.focus({ preventScroll: true });
        field.setSelectionRange(field.value.length, field.value.length);
        return true;
      });
    }
    // withPanel hands an open to the panel lane, window.HousecatChatPanel
    // (ui/dom/chatpanel.ts), with lane as its OpenOptions: the lane keeps
    // the chat the rail already holds and fetches any other, then calls
    // ready with the composer once a frame until ready answers true, for
    // five seconds at most, and a close or a newer open ends it. The lane
    // comes with ui.js, which is deferred, and deferred scripts run
    // before DOMContentLoaded, so an open made while the page is still
    // parsing waits for it, unless the URL has left the chat by then. A
    // page whose bundle never ran fetches the panel with nothing waiting
    // on it (fetchPanel).
    function withPanel(
      id: string,
      lane: OpenOptions,
      ready: (field: HTMLInputElement | HTMLTextAreaElement) => boolean,
    ) {
      if (!window.HousecatChatPanel && document.readyState === "loading") {
        document.addEventListener(
          "DOMContentLoaded",
          () => {
            if (new URL(location.href).searchParams.get(CHAT) === id) withPanel(id, lane, ready);
          },
          { once: true },
        );
        return;
      }
      if (window.HousecatChatPanel) window.HousecatChatPanel.open(id, ready, lane);
      else if ("htmx" in window) fetchPanel(id);
    }
    // fetchPanel is the open without the lane. It aborts the fetch it made
    // last itself, with the request's own abort, held from
    // htmx:config:request as the lane holds its own: the panel's hx-sync
    // replace aborts through the panel queue's record of the request in
    // flight, which a replaced request's late finally clears, so a third
    // open would otherwise run beside the second, and an htmx:abort on
    // the panel would reach neither.
    let fallbackAbort: (() => void) | null = null;
    function fetchPanel(id: string) {
      if (fallbackAbort) fallbackAbort();
      fallbackAbort = null;
      const panel = document.getElementById(CONTENT);
      function capture(e: DocumentEventMap["htmx:config:request"]) {
        const ctx = e.detail?.ctx;
        if (ctx?.sourceElement === panel) fallbackAbort = () => ctx.request.abort();
      }
      document.addEventListener("htmx:config:request", capture);
      window.htmx
        .ajax(
          "GET",
          CHAT_PATH
            + "?"
            + new URLSearchParams([
              ["surface", "mail"],
              [CHAT, id],
            ]).toString(),
          { target: "#" + CONTENT, swap: "outerHTML" },
        )
        .catch(() => {
          /* empty */
        });
      document.removeEventListener("htmx:config:request", capture);
    }
    // railHoldsFocus and handFocusOut move focus out of a closing rail to
    // its toggle, the one part that stays on screen once the panel it was
    // in hides, or below docked, where the toggle is not drawn, to the
    // bar's Mail item.
    function railHoldsFocus() {
      const el = rail();
      return !!el && el.contains(document.activeElement);
    }
    function handFocusOut() {
      const el = rail();
      let toggle = el?.querySelector<HTMLElement>("[data-chat-rail-toggle]");
      if (!toggle || !toggle.checkVisibility()) {
        toggle = document.querySelector<HTMLElement>(
          '#mail-shell-switch [data-shell-switch="mail"]',
        );
      }
      if (toggle && document.activeElement !== toggle) toggle.focus({ preventScroll: true });
    }
    // housecatChatRail is how mail scripts outside the rail open and close
    // it, and what New chat's disabled binding asks (newChatEmpty):
    // open("new", { draft }) starts a new chat with the draft in its
    // composer, which is how "chat about this email" entry points hand the
    // rail their context, and close() closes the open chat, answering
    // whether there was one. Below docked, a chat view pushes opened closes
    // by going back to the page under it, so Escape and Android's back
    // gesture pop the entries the open pushed, as the browser's Back does,
    // and popstate repaints; one opened by the page load has no entry to
    // pop and is replaced, as it always is docked.
    window.housecatChatRail = {
      close() {
        if (!new URL(location.href).searchParams.has(CHAT)) return false;
        const inside = railHoldsFocus();
        const depth = chatDepth();
        if (compactQuery.matches && depth > 0) {
          handOutOnPop = inside || !!required(document.activeElement).closest<HTMLElement>("#mail-shell-switch");
          history.go(-depth);
          return true;
        }
        closeChat();
        if (inside) handFocusOut();
        return true;
      },
      newChatEmpty,
      open(id, options) {
        id ||= "new";
        openChat(id, true, { draft: options?.draft || "", focus: true, fresh: id === "new" });
      },
    };
    chatRailAPI = window.housecatChatRail;
    // A tour step pointing into the rail asks for it with tour:reveal.
    // "new-chat" opens a new chat unless one is already open, whose
    // prompts are what the step is waiting on. "chat" is the chat the
    // user has going, whose answer the step points at, so any open chat
    // is left alone, and with none open the last one comes back.
    document.addEventListener("tour:reveal", (e) => {
      const reveal = e.detail.reveal;
      const open = new URL(location.href).searchParams.get(CHAT);
      if (reveal === "new-chat" && open !== "new") {
        openChat("new", true, { draft: "", focus: false });
      } else if (reveal === "chat" && !open) {
        openChat(lastChat(), true, { draft: "", focus: false });
      }
    });
    // cancelOpen ends an open still under way: the lane aborts the
    // panel's request and stops waiting on the composer. Without the
    // lane it aborts fetchPanel's last fetch.
    function cancelOpen() {
      if (window.HousecatChatPanel) {
        window.HousecatChatPanel.close();
        return;
      }
      if (fallbackAbort) fallbackAbort();
      fallbackAbort = null;
    }
    // STARTED marks a chat that holds a message, sent, sending or failed:
    // ui/dom/chatpanel.ts's selector of that name, copied here because
    // importing that module would start a second panel lane in this bundle.
    const STARTED = "[data-outgoing], section[data-turn-id]";
    // newChatField is the composer of an empty new chat the rail shows: the
    // rail open at this width -- its compact record below docked, its docked
    // one from docked up -- on ?chat=new, with no thread yet and no message.
    // None otherwise: the rail closed, on a conversation, or on a new chat
    // whose first message is under way or failed, which only a fresh open
    // replaces (chatpanel.ts's mounted), so New chat stays live there.
    function newChatField(): Option.Option<HTMLInputElement | HTMLTextAreaElement> {
      const el = rail();
      const content = document.getElementById(CONTENT);
      if (!el || !content) return Option.none();
      const shown = compactQuery.matches ? isOpen(el) : docked(el) === "expanded";
      const fresh = new URL(location.href).searchParams.get(CHAT) === "new"
        && content.getAttribute("data-selected-thread") === ""
        && !content.querySelector(STARTED);
      if (!shown || !fresh) return Option.none();
      return Option.fromNullishOr(
        content.querySelector<HTMLInputElement | HTMLTextAreaElement>("[data-chat-form] [data-prompt-field]"),
      );
    }
    // newChatEmpty is whether New chat has nothing to do: the rail shows a
    // new chat and its input holds nothing but space. Every New chat
    // control binds its disabled attribute to it (hx-live), so it reads
    // as disabled and takes no click, tap or key while that holds, and
    // follows the input and the chat as they change.
    function newChatEmpty(): boolean {
      return Option.match(newChatField(), {
        onNone: () => false,
        onSome: (field) => Str.isEmpty(Str.trim(field.value)),
      });
    }
    // restartNewChat is New chat on a new chat the rail already shows: it
    // empties the input and puts the cursor there, and starts nothing --
    // no navigation, no request. It answers whether the rail showed one.
    // The cursor goes to the composer's editor once a pasted link has laid
    // one over the field (chatcomposer/links.ts), which hides the field.
    function restartNewChat(): boolean {
      return Option.match(newChatField(), {
        onNone: () => false,
        onSome: (field) => {
          if (field.value !== "") {
            field.value = "";
            field.dispatchEvent(new Event("input", { bubbles: true }));
          }
          const editor = field.form?.querySelector<HTMLElement>("[data-slot=chat-composer-editor]");
          (editor ?? field).focus({ preventScroll: true });
          return true;
        },
      });
    }
    // closeChat replaces the entry unless push asks for a new one: the
    // bar's Mail push, so Back slides the chat view in again, as Back
    // after its Chat slides it out. Either entry is the page main
    // shows and is htmx's own, so a later Back onto it has the content
    // lane fetch that page again when main has moved on, and cancels
    // the restore while main still shows it (ui/dom/content.ts). The
    // replaced entry has nothing left to go back to, so its depth goes.
    // It cancels an open still under way, so no panel lands under the
    // closed view.
    function closeChat(push?: boolean) {
      cancelOpen();
      const url = new URL(location.href);
      remember(url.searchParams.get(CHAT));
      url.searchParams.delete(CHAT);
      const closed: Record<string, unknown> = { [CHAT_DEPTH]: undefined };
      if (push === true) pushEntry({}, url);
      else replaceEntry(closed, url);
      markRecent("");
      paint(false);
    }
    document.addEventListener("click", (e) => {
      if (!(e.target instanceof Element)) return;

      const open = e.target.closest<HTMLElement>("[data-chat-open]");
      if (open) {
        e.preventDefault();
        const id = open.getAttribute("data-chat-id") || "new";
        if (id === "new" && restartNewChat()) return;
        openChat(id, true, { focus: true, fresh: id === "new" });
        return;
      }
      if (e.target.closest<HTMLElement>("[data-chat-about-thread]")) {
        e.preventDefault();
        openChat("new", true, { focus: true, fresh: true });
        return;
      }
      if (e.target.closest<HTMLElement>("[data-chat-rail-toggle]")) {
        e.preventDefault();
        if (new URL(location.href).searchParams.has(CHAT)) closeChat();
        else openChat(lastChat(), true, { focus: true });
        return;
      }
      // The bar's Mail | Chat switch is links that navigate on pages
      // without the rail; here, where this script runs, it slides the
      // views instead, each with its own history entry. Mail with chat
      // already closed has nothing to slide: on a mail page it stays
      // put, over the settings card it closes the card as the card's
      // own close link does, and on any other page with the rail it is
      // left to its link, the mail home. A modified click is left to
      // the link, as any link's would be.
      const item = e.target.closest<HTMLElement>("#mail-shell-switch [data-shell-switch]");
      if (!item || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      const chatOpen = new URL(location.href).searchParams.has(CHAT);
      const toChat = item.getAttribute("data-shell-switch") === "chat";
      const sw = item.closest<HTMLElement>("#mail-shell-switch");
      const card = settingsCard();
      const close = card?.querySelector<HTMLElement>("[data-slot=settings-close]");
      if (!toChat && !chatOpen && close) {
        e.preventDefault();
        close.click();
        return;
      }
      if (!toChat && !chatOpen && required(sw).getAttribute("data-shell-area") !== "mail") return;
      e.preventDefault();
      if (toChat) {
        if (!chatOpen) openChat(lastChat(), true, { focus: false });
      } else if (chatOpen) closeChat(true);
    });
    // Docked, the rail stays open beside whatever mail navigates to, so
    // a navigation that does not name a chat carries the open one.
    // Below docked the chat view covers mail, and what a navigation
    // does to it is decided from the content lane's snapshot of the
    // request whose swap is running (HousecatContent.landing(), taken
    // when the request was issued), never from the live URL, which a
    // later open may have changed. drop-chat: a view picked from
    // outside the rail while chat showed is the user asking for mail,
    // so the chat is dropped and, once the URL is committed, the
    // switch shows Mail; the entry it leaves is htmx's own already,
    // so Back fetches that page's mail again under the chat view it
    // brings back. carry-chat: a request issued before chat opened,
    // or docked, lands under the view and the view stays up.
    // close-chat: a link inside the rail to a mail page reveals Mail;
    // the chat the link was rewritten to carry (links.ts) is stripped
    // from the entry, the view is painted closed and focus goes to
    // the bar's Mail item. Either close cancels an open still under way,
    // as closeChat does. The decision comes before the entry's own
    // chat is read, since a rail link's entry already carries it.
    document.addEventListener("htmx:before:history:update", (e) => {
      if (e.defaultPrevented) return;
      const chat = new URL(location.href).searchParams.get(CHAT);
      if (!chat) return;
      const entry = e.detail?.history;
      if (!entry?.path) return;
      const next = new URL(entry.path, location.href);
      const decision = decideEntry(required(required(e.detail).sourceElement), next);
      if (decision === "close-chat") {
        cancelOpen();
        remember(chat);
        next.searchParams.delete(CHAT);
        entry.path = next.pathname + next.search + next.hash;
        afterUpdate(() => {
          markRecent("");
          paint(false);
          handFocusOut();
        });
        return;
      }
      if (decision === "as-is" || next.searchParams.has(CHAT)) return;
      if (decision === "drop-chat") {
        cancelOpen();
        remember(chat);
        afterUpdate(() => {
          markRecent("");
          paint(false);
        });
        return;
      }
      next.searchParams.set(CHAT, chat);
      entry.path = next.pathname + next.search + next.hash;
    });
    // decideEntry is the chat rule for the entry htmx is about to
    // write. The landing navigation is the one this update belongs to
    // only when the update came from its source; a history update the
    // lane did not record -- a swap into another target, or one made
    // before ui.js is in -- is read from its live source: outside the
    // rail below docked drops, anything else carries.
    function decideEntry(source: Element | null, next: URL) {
      const lane = window.HousecatContent;
      const nav = lane.landing();
      if (nav?.ctx.sourceElement === source) {
        return lane.policy.decide(nav, next.pathname + next.search);
      }
      const el = rail();
      return compactQuery.matches && !(el && source && el.contains(source))
        ? "drop-chat"
        : "carry-chat";
    }
    // afterUpdate runs fn once the entry is committed, so a paint
    // that follows a history update reads the URL it left.
    function afterUpdate(fn: () => void) {
      document.addEventListener(
        "htmx:after:history:update",
        () => {
          fn();
        },
        { once: true },
      );
    }
    // The Chats section's stream morphs its rows in without the active
    // mark, since the server cannot know which chat this page has open.
    document.addEventListener("htmx:sse:after:message", (e) => {
      if (!(e.target instanceof Element)) return;

      if (e.target.querySelector<HTMLElement>("[data-recent-chat]")) {
        markRecent(new URL(location.href).searchParams.get(CHAT) || "");
      }
    });
    // A Back that closes the rail while focus is in it hands focus out as
    // close() does, rather than leaving it on a hidden field. A close that
    // goes back hands focus out here too, not before it goes: Chrome
    // resets focus to the body once a traversal settles unless focus
    // moved while it was under way, which a move made here does.
    document.addEventListener("htmx:after:swap", () => {
      if (document.querySelector<HTMLElement>("[data-recent-chat]")) {
        markRecent(new URL(location.href).searchParams.get(CHAT) || "");
      }
    });
    // followChat brings the panel to the chat a history traversal lands
    // on. The content lane restores nothing for an entry that differs
    // from the page shown only in its chat (ui/dom/content.ts), so the
    // panel is otherwise left on the chat it holds, or the one an open
    // still under way lands, under the URL's. The lane decides from what
    // it holds and is opening; without it, a panel already on the chat
    // cancels fetchPanel's last fetch and any other fetches the chat.
    function followChat(id: string) {
      if (window.HousecatChatPanel) {
        window.HousecatChatPanel.follow(id);
        return;
      }
      const panel = document.getElementById(CONTENT);
      if (!panel) return;
      if (panel.getAttribute("data-selected-thread") === (id === "new" ? "" : id)) cancelOpen();
      else if ("htmx" in window) fetchPanel(id);
    }
    // A Back that closes the view cancels an open in flight, as closeChat
    // does; one onto another chat follows it.
    let handOutOnPop = false;
    window.addEventListener("popstate", () => {
      const params = new URL(location.href).searchParams;
      const open = params.has(CHAT);
      const chat = params.get(CHAT) || "";
      if (!open) cancelOpen();
      else if (chat) followChat(chat);
      const inside = !open && (railHoldsFocus() || handOutOnPop);
      handOutOnPop = false;
      markRecent(chat);
      paint(open);
      if (inside) handFocusOut();
    });
    window.addEventListener("resize", paintToggleInset);
    // The compact rail slides by translate, which no ResizeObserver sees, so
    // the inset is taken again once the slide has finished.
    document.addEventListener("transitionend", (e) => {
      if (e.target === rail()) paintToggleInset();
    });
    window.addEventListener("storage", (e) => {
      if (e.key === LAST) paintBadge();
    });
    // The server renders the switch with the open state already, so this
    // first paint moves nothing on screen. The switch follows the rail in
    // the page and ui.js, which brings HousecatSegmented, is deferred, so
    // the switch is painted again once both are in; deferred scripts run
    // before DOMContentLoaded.
    function paintViewNow() {
      paintView(new URL(location.href).searchParams.has(CHAT));
    }
    paintViewNow();
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", paintViewNow);
    }
    document.addEventListener("htmx:after:swap", (e) => {
      const target = e.detail?.ctx?.target;
      if (target instanceof Element && target.matches("[data-mail-content]")) paintViewNow();
    });
    new MutationObserver(queueBadge).observe(document.documentElement, {
      attributeFilter: ["data-chat-status", "data-chat-id", "data-selected-thread"],
      attributes: true,
      childList: true,
      subtree: true,
    });
    paintBadge();
    paintToggleInset();
    document.addEventListener(hook.dataset.threadCreatedEvent ?? "", (e: Event) => {
      if (e instanceof CustomEvent && isThreadEvent(e.detail)) {
        setChatParam(e.detail.threadID, true);
      }
    });
    document.addEventListener(hook.dataset.threadDeletedEvent ?? "", (_e: Event) => {
      openChat("new", false);
    });
  })();
}
