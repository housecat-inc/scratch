import * as Str from "effect/String";
import { required } from "../../dom/required";
export function mail_navigation(_hook: HTMLElement, _swapped: boolean): void {
  (function() {
    const content = document.querySelector<HTMLElement>("[data-mail-content]");
    if (!content) return;
    function syncTitle() {
      const marker = required(content).querySelector<HTMLElement>("[data-mail-document-title]");
      const title = marker ? Str.trim(marker.getAttribute("data-mail-document-title") || "") : "";
      if (title) document.title = title;
    }
    document.addEventListener("htmx:after:swap", (e) => {
      const target = e.detail?.ctx?.target;
      if (target instanceof Element && target.matches("[data-mail-content]")) syncTitle();
    });
    window.addEventListener("pageshow", syncTitle);
    syncTitle();
  })();
}
