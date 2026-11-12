import * as Predicate from "effect/Predicate";

// A client error report: what the browser tells the server about one failure,
// built from whatever was thrown or rejected. It is pure -- no DOM, no
// listeners -- so reporter.ts can wire it to the page and a unit test can feed
// it failures in Node.
//
// A report never carries a query string, DOM content, form value, message
// body, email address or token. describeFailure keeps an error's name, its
// message with addresses, tokens and identifiers scrubbed, and the frames of
// its stack, each URL cut down to its path, or to its origin when it is
// another site's. The server scrubs what it receives again, and maps the page
// path to its route; scrubbing here keeps what leaves the page small.

export type Severity = "error" | "warning";

// Source is what caught a failure: a global handler (uncaught, rejection),
// the shared runtime (defect, log), or code that caught it on purpose
// (reported).
export type Source = "defect" | "log" | "rejection" | "reported" | "uncaught";

// ReportContext is what a caller adds to a failure it reports: a short
// operation name such as "composer.save-draft", the severity, which is
// warning for a failure to clean up (removing a listener, closing a stream,
// disconnecting an observer), and fields that say nothing about the person
// or their mail -- counts, states, flags.
export interface ReportContext {
  readonly fields?: Readonly<Record<string, boolean | number | string>>;
  readonly operation?: string;
  readonly severity?: Severity;
}

// ReportIn is a ReportContext with what the runtime and the global handlers
// add: where the failure was caught, and the stack of the call that reported
// it, for a failure whose own stack names none of our bundles.
export interface ReportIn extends ReportContext {
  readonly site?: string;
  readonly source?: Source;
}

export interface ClientErrorReport {
  readonly fields: Readonly<Record<string, string>>;
  readonly message: string;
  readonly name: string;
  readonly operation: string;
  readonly path: string;
  readonly requestId: string;
  readonly severity: Severity;
  readonly site: string;
  readonly source: Source;
  readonly stack: string;
  readonly time: number;
}

// PageIn is what a report reads of the page it came from.
export interface PageIn {
  readonly origin: string;
  readonly path: string;
  readonly requestId: string;
  readonly time: number;
}

// bundlePath is where the app serves the bundles' build, as app/bundles'
// Path names it. A stack frame under it is ours.
export const bundlePath = "/assets/bundles/";

const limits = { field: 120, fields: 10, frames: 40, message: 500, operation: 80, stack: 6000 };

// describeFailure builds the report for failure, as page saw it.
export function describeFailure(failure: unknown, input: ReportIn, page: PageIn): ClientErrorReport {
  const stack = frames(stackOf(failure), page.origin);
  return {
    fields: fieldsOf(input.fields),
    message: truncate(scrub(messageOf(failure), page.origin), limits.message),
    name: nameOf(failure),
    operation: truncate((input.operation ?? "").replaceAll(/[^\w.:-]/g, "-"), limits.operation),
    path: scrubPath(page.path),
    requestId: /^[0-9a-f]{32}$/.test(page.requestId) ? page.requestId : "",
    severity: input.severity ?? "error",
    site: hasOwnFrame(stack) ? "" : frames(input.site ?? "", page.origin),
    source: input.source ?? "reported",
    stack,
    time: page.time,
  };
}

// hasOwnFrame says whether stack, whole or as frames returns it, names a
// file of our bundles: an error from a browser extension, another site's
// script or the browser itself names none.
export function hasOwnFrame(stack: string, origin = ""): boolean {
  return stack.split("\n").some((line) => isFrame(line) && ownFile(line, origin));
}

// reportKey is what makes two reports the same failure, for deduplicating
// them within a page.
export function reportKey(report: ClientErrorReport): string {
  const top = (report.stack || report.site).split("\n").find((line) => ownFile(line, "")) ?? "";
  return [report.name, report.message, report.operation, top.trim()].join("\n");
}

// stackOf is failure's stack, if it carries one.
export function stackOf(failure: unknown): string {
  return Predicate.hasProperty(failure, "stack") && Predicate.isString(failure.stack) ? failure.stack : "";
}

function nameOf(failure: unknown): string {
  if (Predicate.hasProperty(failure, "_tag") && Predicate.isString(failure._tag)) return failure._tag.slice(0, 80);
  if (Predicate.isError(failure)) return failure.name.slice(0, 80) || "Error";
  if (
    Predicate.hasProperty(failure, "name") && Predicate.isString(failure.name)
    && Predicate.hasProperty(failure, "message")
  ) {
    return failure.name.slice(0, 80) || "Error";
  }
  if (failure === null) return "Null";
  if (failure === undefined) return "Undefined";
  if (Predicate.isString(failure)) return "String";
  if (Predicate.isNumber(failure) || Predicate.isBoolean(failure) || Predicate.isBigInt(failure)) return "Primitive";
  if (typeof Event !== "undefined" && failure instanceof Event) return "Event";
  return "Object";
}

// messageOf is failure's message. An object that isn't an error is never
// turned into a string, since it can hold anything: an event says only its
// type.
function messageOf(failure: unknown): string {
  if (Predicate.hasProperty(failure, "message") && Predicate.isString(failure.message)) return failure.message;
  if (Predicate.isString(failure)) return failure;
  if (Predicate.isNumber(failure) || Predicate.isBoolean(failure) || Predicate.isBigInt(failure)) {
    return String(failure);
  }
  if (typeof Event !== "undefined" && failure instanceof Event) return failure.type;
  return "";
}

function fieldsOf(fields: ReportContext["fields"]): Record<string, string> {
  const out: Record<string, string> = {};
  if (!fields) return out;
  for (const key of Object.keys(fields).sort().slice(0, limits.fields)) {
    if (!/^[a-z][a-z0-9_]{0,39}$/.test(key)) continue;
    // A caller's optional field can arrive undefined, which the record's type leaves out.
    const value = fields[key] as boolean | number | string | undefined;
    if (value === undefined) continue;
    out[key] = truncate(scrub(String(value), ""), limits.field);
  }
  return out;
}

// frames keeps the lines of stack that are frames, scrubbed, at most
// limits.frames of them. The line a V8 stack starts with repeats the message,
// which the report carries on its own.
function frames(stack: string, origin: string): string {
  return truncate(
    stack
      .split("\n")
      .filter(isFrame)
      .slice(0, limits.frames)
      .map((line) => scrub(line.trim(), origin))
      .join("\n"),
    limits.stack,
  );
}

// isFrame recognises a frame as V8 writes one ("at fn (url:1:2)") and as
// Firefox and Safari do ("fn@url:1:2").
function isFrame(line: string): boolean {
  return /^\s*at\s/.test(line) || /@.*:\d+:\d+\)?\s*$/.test(line)
    || /^\s*[\w$.<>]*@(?:\S+|\[native code\])$/.test(line);
}

function ownFile(line: string, origin: string): boolean {
  return origin ? line.includes(origin + bundlePath) : line.includes(bundlePath);
}

const url = /\b[a-z][a-z\d+.-]*:\/\/[^\s'"()<>[\]{}`]+/gi;
const email = /[\w.+%-]+@[\w-]+(?:\.[\w-]+)+/g;
// A token or identifier: a run of 20 or more letters, digits, - and _ with at
// least one digit, or 16 or more hex digits.
const token = /\b(?=[\w-]*\d)(?=[\w-]*[a-zA-Z])[\w-]{20,}\b|\b[0-9a-fA-F]{16,}\b/g;
// A bundle's file, whose name carries a hash a token could look like.
const bundleFile = /\/assets\/bundles\/[\w.-]+/g;

// scrub takes email addresses, tokens and identifiers out of text, and cuts
// each URL down: one of origin's to its path, without a query or fragment;
// another site's to that site's origin.
export function scrub(text: string, origin: string): string {
  const cut = text.replace(url, (found) => scrubURL(found, origin));
  let out = "";
  let last = 0;
  for (const file of cut.matchAll(bundleFile)) {
    out += scrubWords(cut.slice(last, file.index)) + file[0];
    last = file.index + file[0].length;
  }
  return out + scrubWords(cut.slice(last));
}

function scrubWords(text: string): string {
  return text.replace(email, "[email]").replace(token, "[id]");
}

// scrubURL cuts found down, keeping the line and column a stack frame ends
// it with.
function scrubURL(found: string, origin: string): string {
  const position = /(?::\d+){1,2}$/.exec(found)?.[0] ?? "";
  let parsed: URL;
  try {
    parsed = new URL(found.slice(0, found.length - position.length));
  } catch {
    return "[url]";
  }
  if (parsed.origin !== origin) return `${parsed.protocol}//${parsed.host}`;
  return (parsed.pathname.startsWith(bundlePath) ? parsed.pathname : scrubPath(parsed.pathname)) + position;
}

// scrubPath replaces each segment of a path that identifies something -- a
// number, a hex or base64 identifier, an address -- with :id, as the server's
// route normalisation does.
export function scrubPath(path: string): string {
  return path
    .split(/[?#]/)[0]
    .split("/")
    .map((segment) =>
      /^\d+$/.test(segment) || /^[0-9a-fA-F-]{8,}$/.test(segment) || /[\w-]{20,}/.test(segment) || segment.includes("@")
        ? ":id"
        : segment
    )
    .join("/");
}

function truncate(text: string, max: number): string {
  return text.length > max ? text.slice(0, max - 1) + "…" : text;
}
