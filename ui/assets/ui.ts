import "../dom/template_events";
import "../dom/htmx_actions";
import "../dom/template_scripts/url";
import "../dom/template_scripts/toast";
// The entry point for ui.js, the bundles' ui entry: the one script a layout
// loads to get every component's behavior. Each module installs its behavior
// as it is imported: its keys as key router bindings, layers or observers, and
// anything else as delegated document listeners and DOM observers -- the
// tour, of which a page has one, binds its own root instead. floating comes
// along with the modules that place a popup.
// uncaught comes first so its global handlers report what any module after it
// throws. keys comes next so window.HousecatKeys and the router's listeners
// exist before any component or page script handles a key; it brings keyinfo
// with it.
// pan swallows the click that ends a pan on the window, ahead of every
// document listener, wherever a module sits here.
import "../dom/uncaught";
import "../dom/keys";
import "../dom/viewport";
import "../dom/pill";
import "../dom/pan";
import "../dom/intents";
import "../dom/content";
import "../dom/morph";
import "../dom/history";
import "../dom/chatpanel";
import "../dom/shellswitch";
import "../components/button/button_state";
import "../dom/request";
// scrollmemory comes after collection: a full load's restore marks the row
// the user was on through window.HousecatCollection as it is imported.
import "../components/bottomfade/bottomfade";
import "../components/codex/delete";
import "../components/codex/links";
import "../components/codex/scroll";
import "../components/codex/page_context";
import "../components/codex/outgoing_links";
import "../dom/crossing";
import "../components/drawer/drawer";
import "../components/drawer/sheet";
import "../components/chatcomposer/links";
import "../components/popover/popover";
import "../components/promptinput/promptinput";
import "../components/selectbox/selectbox";
import "../components/segmented/segmented";
import "../components/tooltip/tooltip";

import "../dom/housecat";
import "../dom/request_alert";
import "../dom/htmx_sse_swap";
import "../components/collapsible/collapsible";

import "../dom/template_scripts";

import { composing } from "../dom/keyinfo";
// Inline prompt handlers use this compatibility hook to know ui.js has
// loaded. App modules can import key facts before the UI entry is ready.
window.HousecatKeys.composing = composing;
