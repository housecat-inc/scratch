# Keyboard collections

The headless Go parts and one TypeScript controller manage real DOM focus for
entry lists, reorderable tables, clickable tables, and cell-oriented grids.
They add attributes, not wrappers or styles. Static tables keep their native
semantics and normal tab order.

## Row navigation

Use `Root(label, Props{RowNavigation: true})`, which renders
`data-navigation="row"`, with `Row()`, `Cell()` and
`RowHeader()` on the existing structure. `Trigger()` marks the primary native link or button.
When the row's primary target is a text cell, `PrimaryAction()` marks the
control that Enter activates. Consumers do not add their own arrow handlers
or manage tabindex.

`table.Table` exposes this through `NavigationLabel` and `RowNavigation: true`;
`table.RowLink` supplies the primary link. `uikit.MailList` and `MailRow` use
the same parts. The sidebar configurator uses the same behavior with its Edit
button marked as `PrimaryAction()`. Its ordering controller owns only reordering
and section visibility.

Table bodies each use a headless `RowGroup()` navigation root within the
table's ARIA grid. Header and footer controls keep their normal Tab order.
Tab visits each header action before entering the body's single Tab stop;
arrows within the body never move to the table header.

- Up/Down chooses the previous/next visible row at its primary target, from
  any control in the row that has no arrow keys of its own -- a star a click
  focused, a check box, a button under an own-events boundary. Home/End
  reach the first/last row, as Command+Up/Down do on a Mac; Page Up/Down
  moves ten rows. A field or a widget with arrow keys of its own (a listbox,
  toolbar or tabs) keeps them, as do Shift and Alt with an arrow.
- Left/Right visits all visible, enabled controls in the row, across cells.
  Plain description cells are not extra stops. Focus stops at row edges.
- In a tree of rows, section rows carry `aria-level="1"` and `aria-expanded`
  with the control that toggles them; entries carry level 2. Right on a
  closed section row opens it, and past an open row's last control goes on to
  its first entry. Left at a row's primary target closes an open section row,
  or climbs from an entry to its section row.
- Enter opens the primary link, activates `PrimaryAction()` from the row's text
  target, or activates the focused native control. Merely navigating never
  activates an action.
- Escape returns from any control to the current row's primary target. F2
  optionally moves between that target and another control; it is never
  required.
- There is one Tab stop for the entire collection, following the actual focused
  control. Tab leaves the collection; returning restores that control.
- Text fields and nested widgets keep their own arrow keys. Open menus,
  popovers, dialogs and alert dialogs own focus and Escape. Closing them
  returns focus to their invoker.
- Column headers are announced as headers but never become the current row;
  Up/Down from a header, or a control in one, enter the rows at the current
  one. Body header cells (`RowHeader()`) are row headers and join the
  navigation.
- The row remains `data-current` while focus moves among its controls. Consumers
  use that state to show the row indicator. Removing the focused control returns
  focus to the row; removing the row chooses a neighboring row. A script that
  takes a row out to put it back later removes its `data-current` too, so
  putting it back is a change, and the restored row is current again.
- A row the list scrolls into view under a sticky header stops below it:
  `table.Cell` sets a scroll margin of the header height plus a sticky
  section row height the table may set in `--table-sticky-row-height`.

This follows the row-oriented approach used by
[React Aria GridList](https://react-aria.adobe.com/GridList#keyboard-navigation).
The ARIA grid semantics describe the structure; the shared controller defines
how the non-editable row and its controls are navigated.

## Cell navigation

Omit `RowNavigation` for a cell-oriented grid (`data-navigation="cell"`)
following the [WAI-ARIA APG grid pattern](https://www.w3.org/WAI/ARIA/apg/patterns/grid/).
A cell can mark its primary native link with `Trigger()`. A single button or
link cell focuses that widget directly. Text and multi-control cells focus the
cell; Enter or Right enters their controls. F2 is an alternative.

- Arrows move real focus between cells. Up/Down preserve the column, and
  Left/Right skip text-only cells between controls; Left can return to the row's
  first cell. Hidden cells and disabled rows are skipped.
- Home/End reach row edges; Control+Home/End reach grid edges. Page Up/Down
  move ten rows. Static headers are announced as headers without joining the
  arrow navigation. Table header controls use normal Tab navigation.
- One cell or its primary widget is in the page's Tab sequence. Tab exits the
  grid while navigating. F2 enters a cell's controls; Enter does so when focus
  is on the cell itself. Native links and buttons keep their own activation.
- Right can also enter controls from a cell or move from its primary link to the
  next control; Left reaches a control before the primary link. These shortcuts
  supplement APG's Enter/F2 convention without changing native link activation.
- Inside a cell, Left/Right and Tab visit its controls. Left from the first
  control, or arrowing back onto the primary link, returns to grid navigation.
  Text fields and nested widgets retain their own arrow keys.
  Escape or F2 returns to grid navigation. Menus and dialogs keep their own
  focus behavior, including Escape.
- Closed menu buttons defer Up/Down to the grid while navigating. Enter or Space
  opens the menu; its items then use normal menu navigation.
- Focus a click puts on a control with arrow keys of its own enters its cell,
  as F2 would; a click on any other control leaves the grid navigating, so
  Up/Down and Page Up/Down move rows from it and Escape returns to the cell.
- `utils.OwnEvents()` descendants retain interaction ownership of every key
  but Up/Down and Page Up/Down, which move rows from them too. A cell is the
  ownership boundary; its designated link/button still takes grid arrows
  while navigating, and nearer widget bindings take priority. The controller
  uses the shared `housecat.eventOwner`, and does not stop event propagation.

## Both modes

- Focus entering any cell updates the current row. Mail and navigable tables
  opt into hover tracking (`Props.HoverCurrent`): hovering any part of a row,
  its buttons included, makes it current and moves focus to it, so the keys
  act on the row under the pointer. Hover passes over header rows and open
  popups, and changes nothing while focus is on something that keeps the
  keyboard -- a field, a frame, a popup, or an arrow widget outside the list
  -- since a field outside the list, such as All Tools, acts on the current
  row. A control reached from the keyboard keeps its row; one a click
  focused hands the row back to the pointer, whose hand is on the mouse.
- A page's main list of entries, such as Mail's, opts into
  `Props.EnterFromPage`: Up/Down from anywhere off a list -- the page itself,
  a sidebar link, a plain button -- enter the first visible such list. The
  row indicator sets `--collection-current: shown` wherever it is rendered,
  so where the list draws its current row the first press moves on from
  it, as it would with focus on it; where it does not, below md in Mail,
  focus lands on the current row, or the first or last row with none.
  Fields, widgets with arrow keys of their own and popups keep theirs. Lists
  that do not opt in leave the arrows to scroll the page until Tab enters
  them at their current row.
- `current()` scans only the rows marked `data-current`, so it is cheap
  enough for every hover and key the list sees.

The current row (`data-current`) is the keyboard and command context, not a
bulk selection. Navigation never sets `aria-selected` or changes a checkbox.
Mail's compatibility adapter synchronizes its existing `.focused` consumers
with that same current row. Restoration can call `select(row, {scroll:false})`
without stealing focus from a composer or dialog.

A grid cell preserves the semantics of its links, text and controls. A listbox
option does not support interactive descendants, which is why this component
must not use listbox/option roles.
