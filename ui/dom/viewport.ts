// The visible viewport, for the mail shell and anything sized against it.
//
// ViewportStub (ui/layouts/viewport.templ) publishes the docked queries, and
// the docked widths too narrow for the expanded sidebar with the chat rail
// closed and open, on window.HousecatViewport in <head>; this module joins
// that object.
//
// data-keyboard-open on <html> marks a soft keyboard. Nothing reports one
// directly, so it is inferred: the visual viewport is well short of the
// tallest it has been at this width *and* an editable element has focus. The
// height alone is not enough -- a mobile browser's toolbars collapsing and
// expanding change it too -- and focus alone is not either, since a hardware
// keyboard shows nothing. The tallest height seen, not innerHeight, is the
// reference: iOS Safari shrinks innerHeight along with the visual viewport
// when its keyboard opens, so the two never differ by a keyboard's height.
//
// --mail-viewport-height is the shell's height. With no keyboard it is unset,
// and below docked the body falls back to 100lvh: the largest viewport, down
// to the bottom of the screen behind a floating browser toolbar. The visual
// viewport is shorter than that on iOS, so a shell sized to it stops short
// and leaves a band of blank page below it. How much of the body the toolbar
// covers is published as --viewport-toolbar-inset, which the bottom bar pads
// itself by (ui/css/theme.css --bottombar-inset), so its controls stay above
// the toolbar while what scrolls runs on behind it. With the keyboard open it
// is the visual viewport's height, less whatever of it Safari's floating form
// bar covers (formBarCover, below), so the keyboard pushes the bottom of the
// page up rather than covering it; there is no toolbar then, and no toolbar
// inset.
//
// On a shell page -- a body that clips its overflow -- the body is pinned to
// the screen below docked (position: fixed, in the mail stylesheet), so the
// document has nothing to scroll and nothing here ever scrolls it: a scroll
// reset races iOS's own keyboard animation, and the page is seen to shoot up
// and inch back down. iOS still pans the visual viewport within the layout
// viewport to keep a focused field above its keyboard; that pan,
// visualViewport.offsetTop, is published as --mail-viewport-offset-top, and
// the body moves down by it, so it sits exactly over the visible area
// whatever iOS does. And while the keyboard is open a drag that nothing under
// the finger can scroll -- or that would run a scroller past its end -- is
// held still (holdPage), since iOS would otherwise pan the page under it.
import { forgetStorage, readStorage, writeStorage } from "./failures";

export const KEYBOARD_MIN_PX = 150;

declare global {
  interface HousecatViewportGlobal {
    compact: string;
    docked: string;
    narrow: string;
    narrowWithRail: string;
    setDebug?: (on: boolean) => void;
    update?: () => void;
  }
  interface Window {
    HousecatViewport: HousecatViewportGlobal;
  }
}

// Window declares the object as always there, which it is only from here on.
// Without the stub, these copies of ui/layouts' queries stand in for it;
// TestViewportFallback there pins them to the Go constants.
const stub = window.HousecatViewport as HousecatViewportGlobal | undefined;
const viewport = stub ?? (window.HousecatViewport = {
  compact: "(width < 48rem)",
  docked: "(width >= 48rem)",
  narrow: "(48rem <= width < calc(16rem + 35rem))",
  narrowWithRail: "(48rem <= width < calc(16rem + 400px + 35rem))",
});

export function docked(): MediaQueryList {
  return window.matchMedia(viewport.docked);
}

export function compact(): MediaQueryList {
  return window.matchMedia(viewport.compact);
}

// touch is the touch: CSS variant's screen (ui/css/theme.css): no hover, and
// a coarse pointer.
export function touch(): MediaQueryList {
  return window.matchMedia("(hover: none) and (pointer: coarse)");
}

// narrow and narrowWithRail match a docked viewport with too little room for
// the expanded sidebar beside main's minimum width, with the chat rail closed
// and open. They measure the viewport, not main, so collapsing the sidebar
// cannot flip them.
export function narrow(): MediaQueryList {
  return window.matchMedia(viewport.narrow);
}

export function narrowWithRail(): MediaQueryList {
  return window.matchMedia(viewport.narrowWithRail);
}

function editableFocused(): boolean {
  const el = document.activeElement;
  if (!(el instanceof HTMLElement)) return false;
  if (el.isContentEditable) return true;
  if (el instanceof HTMLTextAreaElement) return !el.readOnly && !el.disabled;
  if (el instanceof HTMLInputElement) {
    const nonText = ["button", "checkbox", "color", "file", "hidden", "image", "radio", "range", "reset", "submit"];
    return !el.readOnly && !el.disabled && !nonText.includes(el.type);
  }
  return false;
}

// tallest is the unscaled visual viewport's greatest height at width, the
// height with no keyboard showing. A new width -- a rotation, a resized
// window -- starts it again.
let tallest = 0;
let tallestWidth = 0;

// shellPage is a page whose body is the viewport's size and clips its
// overflow, so any scroll of the document is the browser's, not the reader's.
function shellPage(): boolean {
  const overflow = getComputedStyle(document.body).overflowY;
  return overflow === "hidden" || overflow === "clip";
}

// toolbarInset publishes how much of the body a floating browser toolbar
// covers: the body runs to the bottom of the largest viewport, behind the
// toolbar, while the visible area ends at visibleBottom, above it. Both are
// in the layout viewport's coordinates. Where the visible area reaches the
// body's bottom it is 0, and the home indicator's inset is all the bottom bar
// keeps clear of.
function toolbarInset(visibleBottom: number): void {
  const root = document.documentElement;
  const covered = Math.round(document.body.getBoundingClientRect().bottom - visibleBottom);
  if (covered > 1) root.style.setProperty("--viewport-toolbar-inset", covered + "px");
  else root.style.removeProperty("--viewport-toolbar-inset");
}

// Safari's form bar is the ˄ ˅ ✓ strip it shows over the keyboard for a
// focused field. From iOS 26 it floats clear of the keyboard as a glass
// capsule, and the visual viewport runs on under it down to the keyboard, so
// the bar covers the foot of what the page is told it can see: on an iPhone
// 17 Pro the visual viewport ends at 546pt, where the keyboard starts, and the
// bar spans 488pt to 536pt. Before iOS 26 the bar is joined to the keyboard
// and the visual viewport stops above it, so there is nothing to take off.
// Nothing reports the bar, so the shell leaves FORM_BAR_PX out of the visible
// height itself wherever the bar floats: on an iPhone whose WebKit is iOS 26's
// or later. An iPad keeps those controls in its keyboard's shortcut row, and
// asks for the desktop layout anyway. The OS is dated by its WebKit's
// features rather than the user agent, whose OS version iOS 26 froze and
// which a home-screen app gives no Safari version: every iPhone browser and
// home-screen app runs the system's WebKit, and anchor positioning first
// ships in iOS 26's.
export const FORM_BAR_PX = 58;

function formBarFloats(): boolean {
  return /iPhone|iPod/.test(navigator.userAgent) && typeof CSS !== "undefined" && CSS.supports("anchor-name", "--bar");
}

// formBarCover is how much of the visible area, in CSS pixels, the floating
// form bar covers: its height on screen, larger in CSS pixels at a zoom below
// 1 and smaller above it.
function formBarCover(scale: number): number {
  return formBarFloats() ? FORM_BAR_PX / scale : 0;
}

function measure(): void {
  frame = 0;
  try {
    publish();
  } finally {
    paintDebug();
  }
}

function publish(): void {
  const vv = window.visualViewport;
  const visible = (vv?.height ?? 0) || window.innerHeight;
  if (!visible || visible < 1) return;
  const root = document.documentElement;
  // Pinch-zoom shrinks visualViewport.height by the zoom scale even though no
  // keyboard opened, so the keyboard test undoes the scale back to the
  // unscaled visible height before comparing.
  const scale = (vv?.scale ?? 0) || 1;
  const unscaledVisible = visible * scale;
  if (window.innerWidth !== tallestWidth) {
    tallestWidth = window.innerWidth;
    tallest = 0;
  }
  tallest = Math.max(tallest, window.innerHeight, unscaledVisible);
  const keyboard = tallest - unscaledVisible > KEYBOARD_MIN_PX && editableFocused();
  root.toggleAttribute("data-keyboard-open", keyboard);
  holdTouches(keyboard && shellPage());
  const unzoomed = Math.abs(scale - 1) <= 0.01;
  if (!keyboard) {
    root.style.removeProperty("--mail-viewport-height");
    root.style.removeProperty("--mail-viewport-offset-top");
    if (unzoomed) toolbarInset((vv?.offsetTop ?? 0) + visible);
    return;
  }
  // The toolbar goes when the keyboard comes, but the frames of the
  // keyboard's slide before it counts as one measured a toolbar of their own.
  root.style.removeProperty("--viewport-toolbar-inset");
  root.style.setProperty("--mail-viewport-height", Math.round(visible - formBarCover(scale)) + "px");
  // A zoomed page's pan is the reader's, not the keyboard's.
  const top = unzoomed && shellPage() ? vv?.offsetTop ?? 0 : 0;
  root.style.setProperty("--mail-viewport-offset-top", Math.max(0, Math.round(top)) + "px");
}

// The viewport debug readout: what iOS actually reports, for a developer
// holding the phone. ?viewportdebug=1 turns it on for the rest of the tab's
// session (sessionStorage). The account menu's Viewport readout switch, in
// developer mode, turns it on in this browser until switched off
// (localStorage), which is the only way in for a home-screen app: it has no
// address bar, launches at the manifest's start_url, and keeps a
// sessionStorage of its own. The stored flag counts only on a page drawn with
// developer mode on, the only page that offers the switch to turn it off
// again. ?viewportdebug=0 and the switch turn both off.
// It is a small fixed box in the middle of the screen that takes no pointer
// events, redrawn on every measure() -- visual viewport resize and scroll,
// window resize and scroll, focus moving -- so it follows the keyboard and
// the toolbar live. The safe-area insets and the tokens built from them are
// read off probes (an element padded by env(), one sized by each token),
// since a custom property's own value is a calc() the page never resolves.
// Off, nothing is added to the page.
export const DEBUG_PARAM = "viewportdebug";
const DEBUG_KEY = "housecat:viewportdebug";

function forgetDebug(): void {
  forgetStorage("session", DEBUG_KEY);
  forgetStorage("local", DEBUG_KEY);
}

// developerMode is the data-developer-mode the document draws on <html> for
// a user with developer mode on.
function developerMode(): boolean {
  return document.documentElement.hasAttribute("data-developer-mode");
}

function debugEnabled(): boolean {
  const param = new URLSearchParams(window.location.search).get(DEBUG_PARAM);
  // Storage that is unavailable leaves only the URL to say.
  if (param === "1") writeStorage("session", DEBUG_KEY, "1");
  if (param === "0") forgetDebug();
  if (param === "1" || readStorage("session", DEBUG_KEY) === "1") return true;
  return developerMode() && readStorage("local", DEBUG_KEY) === "1";
}

type Debug = { box: HTMLElement; inset: HTMLElement; tokens: Record<string, HTMLElement> };
let debug: Debug | null = null;
const DEBUG_TOKENS = ["--topbar-height", "--bottombar-height", "--bottombar-inset"];

function debugProbe(style: string): HTMLElement {
  const el = document.createElement("div");
  el.setAttribute("aria-hidden", "true");
  el.style.cssText = "position:fixed;top:0;left:0;width:0;visibility:hidden;pointer-events:none;" + style;
  return el;
}

function mountDebug(): Debug {
  const root = document.documentElement;
  const box = document.createElement("pre");
  box.setAttribute("data-viewport-debug", "");
  box.setAttribute("aria-hidden", "true");
  box.style.cssText = [
    "position:fixed",
    "top:50%",
    "left:50%",
    "z-index:2147483647",
    "translate:-50% -50%",
    "margin:0",
    "padding:8px 10px",
    "max-width:calc(100vw - 16px)",
    "border-radius:8px",
    "background:rgb(0 0 0 / 0.78)",
    "color:#fff",
    "font:11px/1.35 ui-monospace,SFMono-Regular,Menlo,monospace",
    "white-space:pre",
    "pointer-events:none",
  ].join(";");
  const inset = debugProbe(
    "height:0;padding:env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left);",
  );
  const tokens: Record<string, HTMLElement> = {};
  for (const name of DEBUG_TOKENS) tokens[name] = debugProbe(`height:var(${name}, 0px);`);
  root.append(box, inset, ...Object.values(tokens));
  return { box, inset, tokens };
}

function unmountDebug(parts: Debug): void {
  parts.box.remove();
  parts.inset.remove();
  for (const name of DEBUG_TOKENS) parts.tokens[name].remove();
}

function px(n: number): string {
  return Math.round(n * 10) / 10 + "px";
}

function paintDebug(): void {
  if (!debug) return;
  const root = document.documentElement;
  const vv = window.visualViewport;
  const inset = getComputedStyle(debug.inset);
  const lines = [
    "safe-area top/right/bottom/left",
    `  ${inset.paddingTop} ${inset.paddingRight} ${inset.paddingBottom} ${inset.paddingLeft}`,
    `inner ${window.innerWidth}x${window.innerHeight}`,
    vv
      ? `visual ${px(vv.width)}x${px(vv.height)} top ${px(vv.offsetTop)} scale ${Math.round(vv.scale * 1000) / 1000}`
      : "visual (none)",
    `screen.height ${window.screen.height}`,
    `clientHeight ${root.clientHeight}`,
    `body ${px((document.body as HTMLElement | null)?.getBoundingClientRect().height ?? 0)}`,
    `--mail-viewport-height ${root.style.getPropertyValue("--mail-viewport-height") || "(unset)"}`,
    `--viewport-toolbar-inset ${root.style.getPropertyValue("--viewport-toolbar-inset") || "(unset)"}`,
    ...DEBUG_TOKENS.map((name) => `${name} ${px(debug!.tokens[name].getBoundingClientRect().height)}`),
    `data-keyboard-open ${root.hasAttribute("data-keyboard-open")}`,
    `form bar ${formBarFloats() ? `floats, ${px(formBarCover((vv?.scale ?? 0) || 1))} left out` : "nothing left out"}`,
    `standalone ${window.matchMedia("(display-mode: standalone)").matches}`,
    `scrollingElement.scrollTop ${px(document.scrollingElement?.scrollTop ?? 0)}`,
    vv ? `visual pageTop ${px(vv.pageTop)}` : "visual pageTop (none)",
  ];
  debug.box.textContent = lines.join("\n");
}

if (debugEnabled()) debug = mountDebug();

// The account menu's Viewport readout switch. The server renders it off,
// since the setting lives in this browser, so its aria-checked is set here
// as a popover holding it opens: on beforetoggle, ahead of the popover's
// first frame, so a switch drawn since the last set never slides into place
// in front of the user.
const READOUT_SWITCH = "[data-action=viewport-readout]";

function showReadoutState(scope: ParentNode): void {
  for (const item of scope.querySelectorAll(READOUT_SWITCH)) item.setAttribute("aria-checked", String(debug !== null));
}

// setDebug switches the readout in this browser at once, with no reload.
// Storage that refuses the write still switches it for this page.
export function setDebug(on: boolean): void {
  // Storage that refuses keeps nothing; the readout on this page still
  // follows the switch.
  if (on) writeStorage("local", DEBUG_KEY, "1");
  else forgetDebug();
  if (on && !debug) debug = mountDebug();
  if (!on && debug) {
    unmountDebug(debug);
    debug = null;
  }
  paintDebug();
  showReadoutState(document);
}

viewport.setDebug = setDebug;
showReadoutState(document);
document.addEventListener("click", (event) => {
  if (event.target instanceof Element && event.target.closest(READOUT_SWITCH)) setDebug(!debug);
});
document.addEventListener(
  "beforetoggle",
  (event) => {
    if (event.newState === "open" && event.target instanceof Element) showReadoutState(event.target);
  },
  true,
);

// scrollsFor is whether the scroller nearest el, if any, can move under a
// drag of dy (positive: the finger moving down, toward the scroller's top).
// A field the finger is in is left to the browser: it has its own caret and
// selection drags.
function scrollsFor(el: Element | null, dy: number): boolean {
  for (; el && el !== document.body && el !== document.documentElement; el = el.parentElement) {
    if (el.matches("input, textarea, select, [contenteditable]:not([contenteditable=false])")) return true;
    const overflow = getComputedStyle(el).overflowY;
    if ((overflow !== "auto" && overflow !== "scroll") || el.scrollHeight <= el.clientHeight + 1) continue;
    if (dy > 0) return el.scrollTop > 0;
    if (dy < 0) return el.scrollTop + el.clientHeight < el.scrollHeight - 1;
    return true;
  }
  return false;
}

// holdPage keeps a drag from panning the page while the keyboard is open:
// iOS's layout viewport stays the full screen behind the keyboard, so a drag
// that no scroller under the finger takes -- or that would run one past its
// end -- pans the whole page up and lets it inch back down. The transcript,
// or any list, still scrolls under the finger. A touchmove listener that can
// cancel makes every scroll wait on the page's script, so it is only
// attached while it can act -- the keyboard open on a shell page --
// and removed again once the keyboard goes (holdTouches, from publish()).
let touchY = 0;
let holding = false;
function holdTouches(on: boolean): void {
  if (on === holding) return;
  holding = on;
  if (on) document.addEventListener("touchmove", holdPage, { passive: false });
  else document.removeEventListener("touchmove", holdPage);
}
function holdPageStart(event: TouchEvent): void {
  if (event.touches.length === 1) touchY = event.touches[0].clientY;
}
function holdPage(event: TouchEvent): void {
  if (event.touches.length !== 1) return;
  const y = event.touches[0].clientY;
  const dy = y - touchY;
  touchY = y;
  if (!event.cancelable || !document.documentElement.hasAttribute("data-keyboard-open") || !shellPage()) return;
  if (!scrollsFor(event.target instanceof Element ? event.target : null, dy)) event.preventDefault();
}

let frame = 0;
function schedule(): void {
  if (frame) return;
  frame = requestAnimationFrame(measure);
}

viewport.update = measure;

measure();
window.addEventListener("resize", schedule, { passive: true });
window.addEventListener("orientationchange", () => {
  schedule();
  setTimeout(schedule, 80);
  setTimeout(schedule, 240);
}, { passive: true });
// The visual viewport's own events are answered at once rather than on the
// next frame, so the body keeps up with iOS panning the visible area while
// its keyboard slides in.
function now(): void {
  if (frame) cancelAnimationFrame(frame);
  measure();
}
window.visualViewport?.addEventListener("resize", now, { passive: true });
window.visualViewport?.addEventListener("scroll", now, { passive: true });
window.addEventListener("scroll", schedule, { passive: true });
document.addEventListener("touchstart", holdPageStart, { passive: true });
// Focus moving onto or off a field changes the answer without any resize:
// tapping from one field to the next keeps the keyboard, tapping out drops it.
document.addEventListener("focusin", schedule);
document.addEventListener("focusout", schedule);
