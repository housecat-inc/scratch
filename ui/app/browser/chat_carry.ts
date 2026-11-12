const CHAT = "chat";

function docked(): boolean {
  return document.querySelector("[data-chat-rail]")?.getAttribute("data-docked") === "expanded";
}

export function carryChat(): void {
  document.addEventListener("click", (e) => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    if (!(e.target instanceof Element)) return;
    const link = e.target.closest<HTMLAnchorElement>("a[href]");
    if (!link || link.closest("[data-chat-rail]") || link.hasAttribute("download")) return;
    if (link.target && link.target !== "_self") return;
    const chat = new URL(location.href).searchParams.get(CHAT);
    if (!chat || !docked()) return;
    const url = new URL(link.href);
    if (url.origin !== location.origin || url.searchParams.has(CHAT)) return;
    url.searchParams.set(CHAT, chat);
    link.href = url.href;
  });
}
