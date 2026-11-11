import { openerOf } from "../components/menu/menu";
import { CARD_ITEM_CLASS, CARD_LIST_CLASS, cardOpenFor, closeCard, openCard } from "./rowmore";
import { docked } from "./viewport";

// Below docked, a menu whose Root asks for it (menu.RootProps.Card) opens as
// the modal card a mail row's More opens (ui/dom/rowmore.ts) rather than as
// its popup: the phone sheet's account menu and label menu. A popup there is
// a native popover=auto, whose light dismiss lets the press that dismisses it
// go on to whatever lies under it -- a row of the sheet, which navigates --
// and which runs off a short landscape screen. The card's scrim takes that
// press, and it scrolls within the screen.
//
// The popup is never shown: its opening is cancelled at beforetoggle, so
// every way of opening it -- the trigger, an opener (menu:before-open has
// already filled the popup in for the opener), a key -- opens the card
// instead. The card lists the popup's items as the popup has them then, with
// their groups, group labels and separators, leaving out whatever is hidden.
// Choosing a line closes the card and clicks the popup's own item, so
// whatever answers a click on that item -- a form it submits, a link, an htmx
// request, a script's own listener -- answers the choice the same way. An
// item that keeps its menu open (data-prevent-close), such as a switch, keeps
// the card open and redraws its line's state.

const ROOT = "[data-slot=menu][data-menu-card]";
const TRIGGER = "[data-slot=menu-trigger]";
const POSITIONER = "[data-slot=menu-positioner]";
const POPUP = "[data-slot=menu-popup]";

// Classes are written out whole so Tailwind finds them here.
const TITLE_CLASS = "border-b border-border px-4 py-3 type-body-300-medium break-words text-foreground";
const GROUP_LABEL_CLASS = "px-4 pt-2 pb-1 type-caption-100-medium text-muted-foreground";
const SEPARATOR_CLASS = "my-1.5 h-px bg-border";
// A line drawn from a menu item takes the item's group name too, so what the
// item draws from its own state -- a radio's check, a switch's knob, both
// keyed on aria-checked -- draws from the line's.
const LINE_CLASS = CARD_ITEM_CLASS + " group/menu-item disabled:pointer-events-none disabled:opacity-50";

// The roles a menu's stateful items have, as a line in the card has them.
const LINE_ROLE: Record<string, string> = { menuitemcheckbox: "switch", menuitemradio: "radio" };

// copyOf is a copy of part of an item for its line: the same look, with no
// id that would repeat the item's.
function copyOf(node: Node): Node {
  const copy = node.cloneNode(true);
  if (copy instanceof Element) {
    for (const el of [copy, ...copy.querySelectorAll("[id]")]) el.removeAttribute("id");
  }
  return copy;
}

function mirror(line: HTMLElement, item: HTMLElement): void {
  const checked = item.getAttribute("aria-checked");
  if (checked !== null) line.setAttribute("aria-checked", checked);
}

function lineFor(item: HTMLElement): HTMLElement {
  const line = document.createElement("button");
  line.type = "button";
  line.className = LINE_CLASS;
  line.setAttribute("data-slot", "menu-card-item");
  const role = LINE_ROLE[item.getAttribute("role") ?? ""];
  if (role) line.setAttribute("role", role);
  mirror(line, item);
  if (item.hasAttribute("data-disabled")) line.disabled = true;
  for (const child of Array.from(item.childNodes)) line.appendChild(copyOf(child));
  line.addEventListener("click", () => {
    if (item.hasAttribute("data-prevent-close")) {
      item.click();
      mirror(line, item);
      return;
    }
    closeCard();
    // After the close, so focus is back on the opener before the item acts,
    // and a dialog the item opens keeps the focus it takes.
    setTimeout(() => {
      if (item.isConnected) item.click();
    }, 0);
  });
  return line;
}

// draw copies the visible parts of source, in order, into into: a group as a
// group (a radiogroup when it holds radio items) named by its label, which
// is drawn too; a separator as a rule; an item as a line. Anything else, such
// as the form the label menu's items sit in, is passed through.
function draw(source: Element, into: HTMLElement): void {
  for (const part of Array.from(source.children)) {
    if (part instanceof HTMLElement && part.hidden === true) continue;
    switch (part.getAttribute("data-slot") ?? "") {
      case "menu-group": {
        const group = document.createElement("div");
        const radios = part.querySelector("[role=menuitemradio]") !== null;
        group.setAttribute("role", radios ? "radiogroup" : "group");
        const label = part.querySelector("[data-slot=menu-group-label]")?.textContent.trim();
        if (label) group.setAttribute("aria-label", label);
        draw(part, group);
        if (group.querySelector("[data-slot=menu-card-item]")) into.appendChild(group);
        break;
      }
      case "menu-group-label": {
        const label = document.createElement("div");
        label.className = GROUP_LABEL_CLASS;
        label.setAttribute("aria-hidden", "true");
        label.textContent = part.textContent.trim();
        into.appendChild(label);
        break;
      }
      case "menu-separator": {
        const rule = document.createElement("div");
        rule.className = SEPARATOR_CLASS;
        rule.setAttribute("role", "separator");
        into.appendChild(rule);
        break;
      }
      case "menu-item":
        if (part instanceof HTMLElement) into.appendChild(lineFor(part));
        break;
      default:
        draw(part, into);
    }
  }
}

// expanded says whether the card is open on the opener, and on the trigger
// when the opener is another button, as menu.ts says it of the popup.
function expanded(opener: HTMLElement, trigger: HTMLElement, open: boolean): void {
  opener.setAttribute("aria-expanded", String(open));
  if (trigger !== opener) trigger.setAttribute("aria-expanded", String(open));
}

// nameOf is what an opener is called, for the card's accessible name when
// nothing titles it: its aria-label, else the text it shows, part by part --
// the account trigger's name and address are two spans, which textContent
// runs together -- leaving out what is aria-hidden, such as an avatar's
// initials.
function nameOf(el: HTMLElement): string {
  const label = el.getAttribute("aria-label");
  if (label) return label.trim();
  const parts: string[] = [];
  const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const hidden = node.parentElement?.closest("[aria-hidden=true]");
    if (!hidden || !el.contains(hidden)) parts.push(node.textContent ?? "");
  }
  return parts.join(" ").replaceAll(/\s+/g, " ").trim();
}

function open(trigger: HTMLElement, popup: HTMLElement): void {
  const opener = openerOf(trigger);
  if (cardOpenFor(opener)) return;
  const list = document.createElement("div");
  list.className = CARD_LIST_CLASS;
  draw(popup, list);
  // A rule with nothing after it -- the label menu's, over items a label
  // that may only be shown or hidden does not offer -- is left out.
  while (list.lastElementChild?.getAttribute("role") === "separator") list.lastElementChild.remove();
  const parts: Node[] = [];
  const title = opener.dataset.menuTitle ?? "";
  if (title) {
    const heading = document.createElement("div");
    heading.className = TITLE_CLASS;
    heading.setAttribute("data-slot", "menu-card-title");
    heading.textContent = title;
    parts.push(heading);
  }
  parts.push(list);
  // The card gives focus back to whatever held it as it opened: the opener,
  // where a menu gives it back too, even when a long press opened it.
  if (document.activeElement !== opener) opener.focus({ preventScroll: true, focusVisible: false });
  expanded(opener, trigger, true);
  openCard({
    closed: () => expanded(opener, trigger, false),
    label: title || nameOf(opener),
    owner: opener,
    parts,
  });
}

// In the capture phase, so the popup's opening is cancelled whatever else
// listens for it; the card is drawn once every listener has run, since one
// may still be filling the popup in (ui/dom/viewport.ts's readout switch).
document.addEventListener(
  "beforetoggle",
  (event) => {
    const positioner = event.target;
    if (event.newState !== "open" || !(positioner instanceof HTMLElement) || !positioner.matches(POSITIONER)) return;
    const root = positioner.closest<HTMLElement>(ROOT);
    if (!root || docked().matches) return;
    const trigger = root.querySelector<HTMLElement>(TRIGGER);
    const popup = positioner.querySelector<HTMLElement>(POPUP);
    if (!trigger || !popup) return;
    event.preventDefault();
    queueMicrotask(() => open(trigger, popup));
  },
  true,
);
