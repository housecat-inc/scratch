import * as Arr from "effect/Array";
import * as Str from "effect/String";
import { required } from "../../dom/required";
export function content_navigation(_hook: HTMLElement, _swapped: boolean): void {
  (function() {
    if (window.hcContentNavigationBound === true) return;
    window.hcContentNavigationBound = true;
    const verbs = ["hx-action", "hx-delete", "hx-get", "hx-patch", "hx-post", "hx-put"];
    // lane is the content lane once ui.js (deferred) has installed it;
    // the fallback above has no swappable.
    function lane() {
      const l = window.HousecatContent;
      return l;
    }
    // land swaps in the page a post led to: morphed over the page that
    // posted, so the card keeps its scroll, or fresh when it is another.
    // The URL pushed is the lane's entryFor, false when the post came back
    // to the very URL it left from, which a browser doesn't add to history
    // either.
    function land(url: URL) {
      return url.pathname === location.pathname ? "innerMorph" : "innerHTML";
    }

    function driven(el: Element | null) {
      return !!el && Arr.some(verbs, (v) => el.hasAttribute(v));
    }
    // drawable says whether an answer can be drawn over the card: a page
    // with a status below 400, or a card page of any status, which shows
    // what was wrong itself. Anything else -- echo's JSON error, or a
    // proxy's error page while the app restarts -- leaves the card as it
    // was, whether the post answered with it or redirected to it.
    function drawable(res: Response, text?: string) {
      if (!Str.startsWith("text/html")(res.headers.get("Content-Type") || "")) return false;
      return res.status < 400 || Str.includes('data-slot="settings"')(text || "");
    }
    document.addEventListener("click", (e) => {
      if (
        e.defaultPrevented
        || e.button !== 0
        || e.metaKey
        || e.ctrlKey
        || e.shiftKey
        || e.altKey
        || !("htmx" in window)
        || !("HousecatContent" in window)
      ) {
        return;
      }
      const link = e.target instanceof Element ? e.target.closest<HTMLAnchorElement>("a[href]") : null;
      if (!link?.closest<HTMLElement>("[data-slot=settings]")) return;
      if (
        link.hasAttribute("hx-get")
        || link.hasAttribute("download")
        || (link.target && link.target !== "_self")
      ) {
        return;
      }
      const url = lane().swappable(link.href);
      if (!url) return;
      e.preventDefault();
      if (link.getAttribute("data-slot") === "settings-nav-item") {
        document
          .querySelectorAll<HTMLElement>("[data-slot=settings-nav-item][aria-current]")
          .forEach((a) => {
            a.removeAttribute("aria-current");
          });
        link.setAttribute("aria-current", "page");
      }
      lane().visit(url.href);
    });
    document.addEventListener("submit", (e) => {
      const form = e.target;
      const by = e.submitter;
      if (
        e.defaultPrevented
        || !(form instanceof HTMLFormElement)
        || !("htmx" in window)
        || !("HousecatContent" in window)
        || !form.closest<HTMLElement>("[data-slot=settings]")
      ) {
        return;
      }
      if (driven(form) || driven(by)) return;
      if (
        (by?.hasAttribute("formtarget") === true
          ? by.getAttribute("formtarget")
          : form.target || "_self") !== "_self"
      ) {
        return;
      }
      const method = Str.toLowerCase(
        by?.getAttribute("formmethod") || form.getAttribute("method") || "get",
      );
      const url = lane().swappable(
        by?.hasAttribute("formaction") === true
          ? (by.getAttribute("formaction") ?? form.action)
          : form.action,
      );
      if (!url || (method !== "get" && method !== "post")) return;
      e.preventDefault();
      if (method === "get") {
        url.search = new URLSearchParams(
          Array.from(new FormData(form, by), ([key, value]) => [
            key,
            typeof value === "string" ? value : value.name,
          ]),
        ).toString();
        lane().visit(url.href);
        return;
      }
      if (form.classList.contains("htmx-request")) return;
      void window.htmx.ajax("POST", lane().partial(url), {
        cardPost: true,
        event: e,
        source: form,
        swap: "innerMorph",
        target: "[data-mail-content]",
      });
    });
    document.addEventListener("htmx:after:request", (e) => {
      const ctx = e.detail?.ctx;
      if (!(ctx?.cardPost === true)) return;
      const res = required(ctx.response).raw;
      let url;
      if (ctx.hx?.redirect) {
        url = lane().swappable(ctx.hx.redirect);
        if (!url) return;
        ctx.hx.redirect = "";
        ctx.swap = "none";
        void window.htmx.ajax("GET", lane().partial(url), {
          push: lane().entryFor(url),
          swap: land(url),
          target: "[data-mail-content]",
        });
        return;
      }
      if (res.redirected) {
        url = lane().swappable(res.url);
        if (!url) {
          e.preventDefault();
          url = new URL(res.url);
          url.searchParams.delete("_partial");
          location.assign(url.href);
          return;
        }
        if (drawable(res, ctx.text)) {
          ctx.push = lane().entryFor(url);
          ctx.swap = land(url);
          return;
        }
      }
      if (!drawable(res, ctx.text)) {
        ctx.swap = "none";
        return;
      }
      const id = required(ctx.sourceElement).getAttribute("id");
      if (res.status >= 400 && id && required(ctx.sourceElement).isConnected) {
        ctx.select = "#" + CSS.escape(id);
        ctx.swap = "outerMorph";
        ctx.target = ctx.sourceElement;
      }
    });
  })();
}
