// The phone shell's Mail | Chat switch shows which of the stage's two
// layers is up, read at use from the records: the chat view is ?chat in
// the URL, the settings card is [data-mail-content] [data-slot=settings],
// and the rail is whether the page draws one. Without the rail the user is
// in neither view; with it, chat wins, then the card means neither, else
// Mail. The server draws the first paint by the same rule (appBottomBar in
// app/ui/layouts/app_layout.templ), pinned to this one by TestShellSwitchIndex
// and shellswitch.test.ts over one table. ChatRailScript, an inline script,
// reaches it through window.HousecatShellSwitch.

export type ShellSwitchIn = { card: boolean; chat: boolean; rail: boolean };

export function index(s: ShellSwitchIn): -1 | 0 | 1 {
  if (!s.rail) return -1;
  if (s.chat) return 1;
  return s.card ? -1 : 0;
}

declare global {
  interface Window {
    HousecatShellSwitch: { index: typeof index };
  }
}

if (typeof window !== "undefined") window.HousecatShellSwitch = { index };
