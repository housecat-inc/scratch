import * as Arr from "effect/Array";
import * as Str from "effect/String";
function createURL(hook: HTMLElement | null) {
  return (function() {
    function words(name: string) {
      return Str.split(" ")(hook?.getAttribute(name) ?? "");
    }
    const toastParams = words("data-toast-params");
    const transientParams = words("data-mail-transient-params");
    const openerParams = words("data-mail-opener-params");
    const positionParams = words("data-mail-position-params");
    const listParams = words("data-mail-list-params");
    // samePath collapses a navigation target to a path on this origin. An
    // HX-Redirect header, a data-* attribute and a field of a JSON
    // response are values the page did not write, so "javascript:alert(1)"
    // and "//evil.example/" have to come back as a path here or not at
    // all; an absent, cross-origin or unparseable value means the current
    // page.
    //
    // The result is reparsed by whoever assigns it, so returning
    // u.pathname alone is not enough: a same-origin URL may carry a path
    // that itself begins with "//", and "//evil.example/x" assigned to
    // location.href is a host, not a path. Leading slashes collapse to
    // one for that reason, and any edit here has to keep the return value
    // unable to name a host.
    //
    // Query and fragment survive, because the server puts done and
    // done_ids on its redirects to raise the toast on the page it lands
    // on -- a sanitiser that dropped the query would take the toast with
    // it. The base is location.href rather than location.origin so a
    // relative target ("#label=x") still resolves against the current
    // page instead of jumping to the site root.
    function sameOriginURL(raw?: string | null): URL | null {
      let url;
      try {
        url = new URL(raw || location.href, location.href);
      } catch {
        return null;
      }
      if (url.protocol !== "http:" && url.protocol !== "https:") return null;
      return url.origin === location.origin ? url : null;
    }
    function samePath(raw?: string | null) {
      const url = sameOriginURL(raw) ?? new URL(location.href);
      return Str.replace(/^\/+/u, "/")(url.pathname) + url.search + url.hash;
    }
    // externalURL vouches for a target that is meant to leave this origin,
    // such as a CRM record stored on a company. Only http and https reach
    // a new window; "javascript:" and "data:" run in the opener's context
    // when a browser honours them, and no external link needs them. An
    // unusable value returns "", which the caller treats as no link.
    function externalURL(raw?: string | null) {
      let u;
      if (!raw) return "";
      try {
        u = new URL(raw, location.href);
      } catch {
        return "";
      }
      return u.protocol === "http:" || u.protocol === "https:" ? u.href : "";
    }
    // dropToastParams deletes ToastParams from a URLSearchParams.
    function dropToastParams(params: URLSearchParams) {
      Arr.forEach(toastParams, (key) => {
        params.delete(key);
      });
    }
    function dropAll(params: URLSearchParams, keys: string[]) {
      Arr.forEach(keys, (key) => {
        params.delete(key);
      });
    }
    // mailListContext, mailView and namesMailList are
    // layouts.MailListContext, MailView and NamesMailList (mail_url.go),
    // from the groups Go publishes above, so the two cannot drift.
    function mailListContext(raw: string | URLSearchParams) {
      const q = new URLSearchParams(raw);
      dropAll(q, transientParams);
      dropAll(q, toastParams);
      dropAll(q, openerParams);
      Arr.forEach(listParams, (key) => {
        if (q.has(key) && !Str.trim(q.get(key) || "")) q.delete(key);
      });
      return q;
    }
    function mailView(raw: string | URLSearchParams) {
      const q = mailListContext(raw);
      dropAll(q, positionParams);
      return q;
    }
    function namesMailList(params: URLSearchParams) {
      return Arr.some(listParams, (key) => !!Str.trim(params.get(key) || ""));
    }
    return {
      dropToastParams,
      externalURL,
      mailListContext,
      mailView,
      namesMailList,
      sameOriginURL,
      samePath,
    };
  })();
}

export const urlAPI = createURL(
  typeof document === "undefined"
    ? null
    : (document.querySelector<HTMLElement>('template[data-script="url"]')
      ?? document.documentElement),
);
if (typeof window !== "undefined") window.HousecatURL = urlAPI;
