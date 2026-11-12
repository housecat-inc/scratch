import { hasOwnFrame, type ReportContext, stackOf } from "./clienterror";
import { reportError, reportUncaught } from "./runtime";

// The global handlers: one listener for uncaught exceptions and one for
// unhandled rejections, which report through the client error reporter. ui.js
// imports this first, so they listen before any other module runs. A failure
// whose stack names none of our bundles is left alone: a cross-origin
// "Script error.", a browser extension's, another site's script's. Neither
// handler stops an error reaching the console.

declare global {
  // HousecatErrors is how a script that cannot import the kit, such as an
  // inline templ script, reports a failure it caught. Its presence marks the
  // handlers installed, so a second copy of this module, from another build,
  // installs none.
  interface Window {
    HousecatErrors?: { report: (failure: unknown, context?: ReportContext) => void };
  }
}

function install(): void {
  if (window.HousecatErrors) return;
  window.HousecatErrors = { report: reportError };
  window.addEventListener("error", (event) => {
    // An error a script threw carries the file and position it was thrown at
    // even when what it threw is no Error with a stack of its own.
    const site = event.filename ? `at ${event.filename}:${event.lineno}:${event.colno}` : "";
    if (!hasOwnFrame(stackOf(event.error) || site, location.origin)) return;
    reportUncaught(event.error ?? event.message, "uncaught", site);
  });
  window.addEventListener("unhandledrejection", (event) => {
    if (!hasOwnFrame(stackOf(event.reason), location.origin)) return;
    reportUncaught(event.reason, "rejection");
  });
}

if (typeof window !== "undefined") install();
