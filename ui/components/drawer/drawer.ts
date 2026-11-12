// Behaviour for a drawer that owns its compact record: data-control="self"
// on the aside DrawerUnstyled renders. A caller-controlled drawer -- the
// chat rail, whose state is the URL -- is opened and closed by its caller,
// through ./records, and nothing here opens or closes it.
//
// data-drawer-open on the aside is the compact record (./records): the
// styled Drawer's classes, the key layer's match and every script read it
// there, and setOpen writes it with the backdrop and the triggers that
// follow it. data-drawer-modal on the aside is the one record that it opened
// as a modal: open() sets it when it takes focus and close() clears it, and
// everything modal is projected from it. Below docked an open modal drawer
// is a modal dialog: role=dialog and aria-modal on the aside, written here,
// and the rest of the page inert, written by the key router from the layer
// below, which asks to be modal while the record is there and lists the
// backdrop among its regions so a tap on the scrim still closes the drawer.
// A drawer opened without taking focus -- the tour revealing the sidebar --
// is shown, not modal, and leaves the page live, since the tour's own
// controls are in it.
//
// The docked record, data-docked, is ./records' setDocked, published on
// window.HousecatDrawer beside open and close. None of them touches the
// other record.

import * as Arr from "effect/Array";
import * as Option from "effect/Option";

import { compact } from "../../dom/viewport";
import { backdrop, collapsible, docked, isOpen, SELF, setDocked, setOpen } from "./records";

type OpenOptions = { focus?: boolean; opener?: Element | null };
type CloseOptions = { restoreFocus?: boolean };

declare global {
  interface Window {
    HousecatDrawer: {
      close: typeof close;
      collapsible: typeof collapsible;
      docked: typeof docked;
      isOpen: typeof isOpen;
      open: typeof open;
      setDocked: typeof setDocked;
    };
  }
}

const openers = new WeakMap<Element, Element>();

// dialog projects the modal record onto the aside: role=dialog and
// aria-modal while on, neither otherwise. It is on only below docked, where
// the drawer covers the page; from docked up it is part of the layout and
// must not be announced as a dialog.
function dialog(el: Element, on: boolean): void {
  if (on) {
    el.setAttribute("role", "dialog");
    el.setAttribute("aria-modal", "true");
    return;
  }
  el.removeAttribute("role");
  el.removeAttribute("aria-modal");
}

function modalBelowDocked(el: Element): boolean {
  return el.matches(`${SELF}[data-compact=modal]`) && compact().matches;
}

// open writes the compact record and the modal one and then settles the key
// router itself, before it focuses: the router's own settle runs on the
// mutation batch's microtask, after this task, so a tap in the same task
// could otherwise land on a page not yet inert, and the aside may itself be
// inert under another modal layer -- the sheet opening over the chat view --
// until the projection lets it go, where focus() and tabbable() would find
// nothing.
export function open(el: Element, options: OpenOptions = {}): void {
  if (options.opener) openers.set(el, options.opener);
  setOpen(el, true);
  if (options.focus !== false) {
    el.toggleAttribute("data-drawer-modal", true);
    dialog(el, modalBelowDocked(el));
  }
  window.HousecatKeys.settle();
  if (options.focus === false) return;
  const first = window.HousecatKeys.tabbable(el)[0];
  if (first instanceof HTMLElement) first.focus();
}

// close clears the compact record and the modal one and settles the router
// before it hands focus back: the opener sits on the page the projection
// holds inert until then, and focus() into an inert subtree is ignored.
export function close(el: Element, options: CloseOptions = {}): void {
  if (!isOpen(el)) return;
  setOpen(el, false);
  el.removeAttribute("data-drawer-modal");
  dialog(el, false);
  window.HousecatKeys.settle();
  const opener = openers.get(el);
  openers.delete(el);
  if (options.restoreFocus === true && opener instanceof HTMLElement && opener.isConnected) opener.focus();
}

window.HousecatDrawer = { close, collapsible, docked, isOpen, open, setDocked };

// While the window is being resized, and for 120ms after its last resize
// event, every drawer carries data-resizing, which drops its transitions
// (drawer.templ). A drawer's width can follow the viewport -- the chat
// rail's is the stage's whole width below docked, and every drawer changes
// width and position crossing the docked breakpoint -- and its transition
// would otherwise chase each new value with the ease a record change uses,
// so the panel visibly lags the window instead of tracking it. The drawer
// sets it, so no caller has to know which drawers move with the window.
const RESIZE_SETTLE_MS = 120;
let resizeSettle: number | undefined;

window.addEventListener("resize", () => {
  for (const el of document.querySelectorAll("[data-slot=drawer]")) el.toggleAttribute("data-resizing", true);
  window.clearTimeout(resizeSettle);
  resizeSettle = window.setTimeout(() => {
    for (const el of document.querySelectorAll("[data-slot=drawer][data-resizing]")) {
      el.removeAttribute("data-resizing");
    }
  }, RESIZE_SETTLE_MS);
});

document.addEventListener("click", (event) => {
  const target = event.target;
  if (!(target instanceof Element)) return;

  const trigger = target.closest<HTMLElement>("[data-drawer-trigger]");
  if (trigger) {
    const el = document.getElementById(trigger.dataset.drawerTrigger ?? "");
    if (!el || !el.matches(SELF)) return;
    event.preventDefault();
    if (isOpen(el)) close(el, { restoreFocus: true });
    else open(el, { opener: trigger });
    return;
  }

  const closer = target.closest("[data-drawer-close]");
  const closing = closer?.closest(SELF);
  if (closing) {
    event.preventDefault();
    close(closing, { restoreFocus: true });
    return;
  }

  const back = target.closest<HTMLElement>("[data-drawer-backdrop]");
  if (back) {
    const el = document.getElementById(back.dataset.drawerFor ?? "");
    if (el?.matches(SELF) === true) close(el, { restoreFocus: true });
  }
});

// Below docked a followed link closes the drawer it is in, on the click's
// way down to the link, ahead of whatever the link does itself, so the
// navigation lands with the drawer already off the page. It listens on the
// root, not the document, so the window's and the document's capture
// listeners -- the swallowed click that ends a pan (ui/dom/pan.ts) -- run
// first and can stop it.
document.documentElement.addEventListener(
  "click",
  (event) => {
    const target = event.target;
    if (!(target instanceof Element) || target.closest("[data-drawer-trigger], [data-drawer-close]")) return;
    const holder = target.closest("a[href]")?.closest(`${SELF}[data-close-on-navigate]`);
    if (holder && compact().matches) close(holder);
  },
  true,
);

// shown is whether el's contents are in sight at the current width: below
// docked, while its compact record is open; from docked up, unless it is
// collapsed out of sight.
function shown(el: Element): boolean {
  if (compact().matches) return isOpen(el);
  return !(el.getAttribute("data-collapse") === "hidden" && docked(el) === "collapsed");
}

// visible is whether el is drawn and can take focus. It counts visibility,
// since the clip box hides a drawer's contents with visibility: hidden,
// where focus() is silently ignored.
function visible(el: Element): boolean {
  return "checkVisibility" in Element.prototype
    ? el.checkVisibility({ visibilityProperty: true })
    : el.getClientRects().length > 0;
}

// refuge is where focus goes from el's contents when they are no longer
// where it was: el's first control when its contents are in sight, else
// the visible control that opens it -- the bottom bar's navigation button
// for the sidebar folded into its closed sheet -- else nowhere.
function refuge(el: Element): HTMLElement | null {
  const candidates = shown(el)
    ? window.HousecatKeys.tabbable(el)
    : el.id
    ? [...document.querySelectorAll(`[data-drawer-trigger="${CSS.escape(el.id)}"]`)]
    : [];
  return Option.getOrNull(
    Arr.findFirst(candidates, (c): c is HTMLElement => c instanceof HTMLElement && visible(c)),
  );
}

// rehome moves focus out of el when a crossing leaves it on something the
// user can no longer see or reach: anywhere in contents that went out of
// sight, a popover opened from them included, or a control the new width
// hides, such as the sheet's close handle from docked up. Focus that is
// still in sight stays where it is.
function rehome(el: Element): void {
  const active = document.activeElement;
  if (!(active instanceof HTMLElement) || !el.contains(active)) return;
  if (shown(el) && visible(active)) return;
  const to = refuge(el);
  if (to) to.focus({ preventScroll: true });
  else active.blur();
}

// Crossing the docked breakpoint:
//
//   - Up to docked, every open self-controlled drawer closes. The sheet or
//     overlay it opened as is gone at that width -- the sidebar is a column
//     of the layout there -- so its compact record goes back to closed, its
//     modal record and dialog role with it, and crossing back down does not
//     bring it back open. Its docked record is untouched.
//   - Down below docked, an open drawer whose modal record is still on -- one
//     opened from docked up, as a script may -- is a dialog again.
//   - Either way, focus left in contents that went out of sight, or on a
//     control the new width hides, moves to the drawer's refuge.
//
// Every open popover closes on the crossing into compact too (ui/dom/
// crossing.ts); whichever listener runs first, focus ends up out of the
// drawer's hidden contents, since closing a popover hands focus back to its
// trigger in the same task and the trigger is in those contents. The
// router's own listener on the same media re-projects inert, and close()
// settles it.
compact().addEventListener("change", (event) => {
  if (event.matches) {
    for (const el of document.querySelectorAll(`${SELF}[data-compact=modal][data-drawer-open][data-drawer-modal]`)) {
      dialog(el, true);
    }
  } else {
    for (const el of document.querySelectorAll(`${SELF}[data-drawer-open]`)) close(el);
  }
  for (const el of document.querySelectorAll("[data-slot=drawer]")) rehome(el);
});

// Below docked an open modal drawer is a layer: keys stay in it, Escape
// closes it and hands focus back to what opened it, and on a coarse pointer
// Android's back gesture closes it too. An open auto popover inside takes
// Escape first, through the router's Escape order. The first key after an
// open that took no focus lands on the drawer's first control. While the
// aside carries data-drawer-modal the layer is modal and the router holds
// the page outside the aside and its backdrop inert; the record is written
// in the same task as data-drawer-open, which the router watches, and open()
// settles the router itself, so the predicate never reads a stale record.
// A drawer that always sits over another layer, whichever opened last, names
// that layer's id in data-drawer-over on the aside, space separated, and the
// router keeps it above that layer: the drawer itself names no layer of the
// page it is drawn on. The attribute is on only the drawers that set it.
window.HousecatKeys.layer({
  back: true,
  escape: (_event, el) => {
    if (el) close(el, { restoreFocus: true });
    return true;
  },
  id: "self-drawer",
  initial: (el) => {
    const first = el ? window.HousecatKeys.tabbable(el)[0] : undefined;
    return first instanceof HTMLElement ? first : null;
  },
  match: `${SELF}[data-compact=modal][data-drawer-open]`,
  media: compact().media,
  modal: (el) => !!el && el.hasAttribute("data-drawer-modal"),
  over: (el) => (el?.getAttribute("data-drawer-over") ?? "").split(/\s+/).filter(Boolean),
  regions: (el) => (el ? [el, backdrop(el)] : []),
});
