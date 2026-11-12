import * as Arr from "effect/Array";
import * as Rec from "effect/Record";
import * as Str from "effect/String";
import { navSections, saveNavSections } from "../../dom/shellprefs";
export function mail_sidebar(_hook: HTMLElement, _swapped: boolean): void {
  (function() {
    document.addEventListener("tour:reveal", (e) => {
      if (e.detail.reveal !== "sidebar") return;
      if (!window.matchMedia(window.HousecatViewport.compact).matches) return;
      const sidebar = document.querySelector<HTMLElement>("#mail-sidebar");
      if (!sidebar || !("HousecatDrawer" in window) || window.HousecatDrawer.isOpen(sidebar)) {
        return;
      }
      window.HousecatDrawer.open(sidebar, { focus: false });
    });
    // A sidebar section's open state, and a label folder's or the
    // labels' More toggle's, is saved in the shell's sections preference
    // (ui/dom/shellprefs.ts), which the server reads to render it the same
    // way on the next page. Only a click on its header saves it: a
    // navigation morphing the sidebar sets open too and fires the same
    // toggle event, and that must not overwrite the user's choice. Folders
    // start closed, so a closed folder drops its key rather than saving
    // closed, and the preference holds only the folders the user keeps
    // open.
    let clickedSection: HTMLDetailsElement | null = null;
    document.addEventListener(
      "click",
      (e) => {
        if (!(e.target instanceof Element)) return;

        const summary = e.target.closest<HTMLElement>("[data-nav-section] > summary");
        clickedSection = summary?.parentElement instanceof HTMLDetailsElement ? summary.parentElement : null;
      },
      true,
    );
    function isFolder(key: string) {
      return Str.startsWith("L:")(key);
    }
    document.addEventListener(
      "toggle",
      (e) => {
        const details = e.target;
        if (!(details instanceof HTMLDetailsElement)) return;
        if (details !== clickedSection) return;
        clickedSection = null;
        const sections = navSections();
        const key = details.dataset.navSection ?? "";
        saveNavSections(
          !details.open && isFolder(key) ? Rec.remove(sections, key) : Rec.set(sections, key, details.open),
        );
      },
      true,
    );
    // A navigation's sidebar is rendered from the cookie its request
    // carried, so a header clicked while the request was out would be
    // undone by the morph. The request keeps the choices it went out with,
    // and a section whose choice has changed since takes the new one
    // before the sidebar is morphed in. A folder with no key is closed.
    document.addEventListener("htmx:before:request", (e) => {
      if (e.detail?.ctx) e.detail.ctx.navSections = navSections();
    });
    function choiceOf(sections: Record<string, boolean>, key: string): boolean | null {
      return sections[key] ?? (isFolder(key) ? false : null);
    }
    document.addEventListener("htmx:before:swap", (e) => {
      const detail = e.detail;
      const sent = detail?.ctx?.navSections;
      if (!sent || !detail.tasks) return;
      const now = navSections();
      Arr.forEach(detail.tasks, (task) => {
        if (!task.fragment || task.target?.id !== "mail-sidebar-content") return;
        task.fragment
          .querySelectorAll<HTMLElement>("details[data-nav-section]")
          .forEach((details) => {
            const choice = choiceOf(now, details.dataset.navSection ?? "");
            if (choice !== null && choice !== choiceOf(sent, details.dataset.navSection ?? "")) {
              details.toggleAttribute("open", choice);
            }
          });
      });
    });
  })();
}
