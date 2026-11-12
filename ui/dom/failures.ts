import * as Data from "effect/Data";
import * as Effect from "effect/Effect";
import * as Predicate from "effect/Predicate";

import type { ReportContext, ReportIn } from "./clienterror";
import { ClientErrorReporter } from "./reporter";
import { callSite, runPromise, runSync } from "./runtime";

// How browser code handles the failures it expects, and reports the ones it
// doesn't (docs/app-development.md, Browser errors).
//
// Each expected failure is a tagged error, which an Effect handles by tag
// with a fallback of its own. Code that isn't Effect uses the plain function
// beside it, which handles the tag with the fallback its name and comment
// give, and reports nothing. A throw that is none of the failures an
// operation expects is unexpected: the plain functions report it, with the
// operation's name, and still fall back, so what the user sees doesn't
// change.

// StorageUnavailable is storage the browser refuses: blocked, as in some
// private windows and sandboxed frames, or full.
export class StorageUnavailable extends Data.TaggedError("StorageUnavailable")<{ readonly cause: unknown }> {}

// NotJSON is text, or a response's body, that doesn't parse as JSON, such as
// a proxy's HTML error page.
export class NotJSON extends Data.TaggedError("NotJSON")<{ readonly cause: unknown }> {}

// NotAURL is text that doesn't parse as a URL.
export class NotAURL extends Data.TaggedError("NotAURL")<{ readonly cause: unknown }> {}

// CrossOriginFrame is a frame whose document a script may not reach.
export class CrossOriginFrame extends Data.TaggedError("CrossOriginFrame")<{ readonly cause: unknown }> {}

// TaintedCanvas is a canvas another site's image drew on, which refuses to be
// read.
export class TaintedCanvas extends Data.TaggedError("TaintedCanvas")<{ readonly cause: unknown }> {}

// ElementGone is an operation on an element, a range or a pointer that left
// before it ran: a popover shown or hidden once it is out of the document, a
// selection set to a node an edit took away or to an offset past its end, a
// pointer captured once the browser has let it go.
export class ElementGone extends Data.TaggedError("ElementGone")<{ readonly cause: unknown }> {}

// RequestAborted is a request that never completed: aborted or cancelled by
// the page, or cut off by the network or by the page going away, which a
// fetch says with a TypeError of its own.
export class RequestAborted extends Data.TaggedError("RequestAborted")<{ readonly cause: unknown }> {}

// HTTPFailure is an answer the server gave with an error status. The server
// records its own failures, so the browser doesn't report them again.
export class HTTPFailure extends Data.TaggedError("HTTPFailure")<{ readonly status: number }> {}

// Refused is a feature the browser refuses here: one that wants a user
// gesture or a permission it wasn't given, such as a date picker opened from
// script or the clipboard, or that it doesn't support.
export class Refused extends Data.TaggedError("Refused")<{ readonly cause: unknown }> {}

// SchemaMismatch is JSON from the server that doesn't match the Schema
// generated from the Go type that sends it (wire.ts). Its message names each
// field that doesn't match, by its path and the kind of issue, and never
// holds a value: a map's keys and an array's positions read as * and [].
export class SchemaMismatch extends Data.TaggedError("SchemaMismatch")<{
  readonly endpoint: string;
  readonly message: string;
}> {}

// UnexpectedFailure is any other throw, which the plain functions report.
export class UnexpectedFailure extends Data.TaggedError("UnexpectedFailure")<{ readonly cause: unknown }> {}

export type StorageArea = "local" | "session";

// Reading the property can throw too, in a frame whose storage is blocked.
function storage(area: StorageArea): Storage {
  return area === "local" ? window.localStorage : window.sessionStorage;
}

export const storageItem = (area: StorageArea, key: string): Effect.Effect<string | null, StorageUnavailable> =>
  Effect.try({ catch: (cause) => new StorageUnavailable({ cause }), try: () => storage(area).getItem(key) });

export const setStorageItem = (
  area: StorageArea,
  key: string,
  value: string,
): Effect.Effect<void, StorageUnavailable> =>
  Effect.try({ catch: (cause) => new StorageUnavailable({ cause }), try: () => storage(area).setItem(key, value) });

export const removeStorageItem = (area: StorageArea, key: string): Effect.Effect<void, StorageUnavailable> =>
  Effect.try({ catch: (cause) => new StorageUnavailable({ cause }), try: () => storage(area).removeItem(key) });

export const parseJSON = (text: string): Effect.Effect<unknown, NotJSON> =>
  Effect.try({ catch: (cause) => new NotJSON({ cause }), try: (): unknown => JSON.parse(text) });

export const parseURL = (text: string, base?: string | URL): Effect.Effect<URL, NotAURL> =>
  Effect.try({ catch: (cause) => new NotAURL({ cause }), try: () => new URL(text, base) });

export const frameDocument = (frame: HTMLIFrameElement): Effect.Effect<Document | null, CrossOriginFrame> =>
  Effect.try({ catch: (cause) => new CrossOriginFrame({ cause }), try: () => frame.contentDocument });

export const responseJSON = (response: Response): Effect.Effect<unknown, NotJSON | RequestAborted> =>
  Effect.tryPromise({
    catch: (cause) => (isAborted(cause) ? new RequestAborted({ cause }) : new NotJSON({ cause })),
    try: (): Promise<unknown> => response.json(),
  });

export const canvasDataURL = (canvas: HTMLCanvasElement): Effect.Effect<string, TaintedCanvas | UnexpectedFailure> =>
  Effect.try({
    catch: (cause) =>
      isDOMException(cause, ["SecurityError"]) ? new TaintedCanvas({ cause }) : new UnexpectedFailure({ cause }),
    try: () => canvas.toDataURL("image/png"),
  });

// The DOMExceptions that say the browser refuses a feature.
const refusedNames = ["NotAllowedError", "NotSupportedError", "SecurityError"];

export const refusableCall = <A>(f: () => A): Effect.Effect<A, Refused | ElementGone | UnexpectedFailure> =>
  Effect.try({
    catch: (cause) =>
      isDOMException(cause, refusedNames)
        ? new Refused({ cause })
        : isDOMException(cause, goneNames)
        ? new ElementGone({ cause })
        : new UnexpectedFailure({ cause }),
    try: f,
  });

// The DOMExceptions that say an element, range or pointer has gone.
const goneNames = ["IndexSizeError", "InvalidNodeTypeError", "InvalidStateError", "NotFoundError"];

export const elementCall = <A>(f: () => A): Effect.Effect<A, ElementGone | UnexpectedFailure> =>
  Effect.try({
    catch: (
      cause,
    ) => (isDOMException(cause, goneNames) ? new ElementGone({ cause }) : new UnexpectedFailure({ cause })),
    try: f,
  });

// isAborted says whether cause is a request that never completed.
export function isAborted(cause: unknown): boolean {
  return isDOMException(cause, ["AbortError"]) || cause instanceof TypeError && isFetchFailure(cause.message);
}

// A fetch that never got an answer rejects with a TypeError that says so, in
// each engine's words.
function isFetchFailure(message: string): boolean {
  return /^(?:Failed to fetch|Load failed|NetworkError when attempting to fetch resource\.?|Network request failed)$/
    .test(message);
}

// isHTTPFailure says whether cause is an answer with an error status: an
// HTTPFailure, or the Error reading "HTTP <status>" that the islands' request
// helpers throw.
export function isHTTPFailure(cause: unknown): boolean {
  return cause instanceof HTTPFailure || cause instanceof Error && /^HTTP \d{3}$/.test(cause.message);
}

// isGone says whether cause is an element or pointer that left before an
// operation on it ran, as a DOMException says it, or as htmx's ajax does when
// the element it was to swap is no longer on the page.
export function isGone(cause: unknown): boolean {
  return isDOMException(cause, goneNames)
    || cause instanceof Error && /^(?:Source|Target) not found$/.test(cause.message);
}

function isDOMException(cause: unknown, names: ReadonlyArray<string>): boolean {
  return Predicate.hasProperty(cause, "name") && Predicate.isString(cause.name) && names.includes(cause.name)
    && (typeof DOMException === "undefined" || cause instanceof DOMException);
}

// readStorage is the stored value, or null when there is none or storage is
// unavailable.
export function readStorage(area: StorageArea, key: string): string | null {
  return runSync(storageItem(area, key).pipe(Effect.catchTag("StorageUnavailable", () => Effect.succeed(null))));
}

// writeStorage stores value and says whether it could: unavailable storage
// keeps nothing, and the page carries on without it.
export function writeStorage(area: StorageArea, key: string, value: string): boolean {
  return runSync(
    setStorageItem(area, key, value).pipe(
      Effect.as(true),
      Effect.catchTag("StorageUnavailable", () => Effect.succeed(false)),
    ),
  );
}

// forgetStorage removes the value; unavailable storage has none to remove.
export function forgetStorage(area: StorageArea, key: string): void {
  runSync(removeStorageItem(area, key).pipe(Effect.catchTag("StorageUnavailable", () => Effect.void)));
}

// readJSON is text parsed as JSON, or undefined -- which no JSON parses to --
// when there is no text or it isn't JSON.
export function readJSON(text: string | null | undefined): unknown {
  if (text === null || text === undefined) return undefined;
  return runSync(parseJSON(text).pipe(Effect.catchTag("NotJSON", () => Effect.undefined)));
}

// readResponseJSON is response's body parsed as JSON, or undefined when it
// isn't JSON or never fully arrived.
export function readResponseJSON(response: Response): Promise<unknown> {
  return runPromise(
    responseJSON(response).pipe(
      Effect.catchTags({ NotJSON: () => Effect.undefined, RequestAborted: () => Effect.undefined }),
    ),
  );
}

// readURL is text as a URL, relative to base, or undefined when it isn't one.
export function readURL(text: string, base?: string | URL): URL | undefined {
  return runSync(parseURL(text, base).pipe(Effect.catchTag("NotAURL", () => Effect.undefined)));
}

// readFrameDocument is frame's document, or null when the frame has none or
// it is cross-origin.
export function readFrameDocument(frame: HTMLIFrameElement): Document | null {
  return runSync(frameDocument(frame).pipe(Effect.catchTag("CrossOriginFrame", () => Effect.succeed(null))));
}

// readCanvasDataURL is canvas as a PNG data: URL, or "" when another site's
// image tainted it, or reading it failed as nothing expects, which is
// reported.
export function readCanvasDataURL(canvas: HTMLCanvasElement): string {
  return runSync(
    canvasDataURL(canvas).pipe(
      Effect.catchTags({
        TaintedCanvas: () => Effect.succeed(""),
        UnexpectedFailure: (failure) => Effect.as(report(failure.cause, { operation: "canvas.read" }), ""),
      }),
    ),
  );
}

// attached runs f, an operation on an element or pointer that may have gone,
// and says whether it ran. A gone element is expected; any other failure is
// reported as operation.
export const attached = (operation: string, f: () => void): Effect.Effect<boolean, never, ClientErrorReporter> =>
  elementCall(f).pipe(
    Effect.as(true),
    Effect.catchTags({
      ElementGone: () => Effect.succeed(false),
      UnexpectedFailure: (failure) => Effect.as(report(failure.cause, { operation }), false),
    }),
  );

// attempt runs f, and when it throws, which nothing expects, reports the
// failure with context and gives fallback, what the code did before it
// reported anything.
export const attempt = <A>(
  context: ReportContext & { readonly operation: string },
  f: () => A,
  fallback: A,
): Effect.Effect<A, never, ClientErrorReporter> =>
  Effect.try({ catch: (cause) => new UnexpectedFailure({ cause }), try: f }).pipe(
    Effect.catchTag("UnexpectedFailure", (failure) => Effect.as(report(failure.cause, context), fallback)),
  );

// reportingUnless reports cause as input says, unless one of expected says
// it is a failure the caller expects.
export const reportingUnless = (
  input: ReportIn,
  ...expected: ReadonlyArray<(cause: unknown) => boolean>
) =>
(cause: unknown): Effect.Effect<void, never, ClientErrorReporter> =>
  expected.some((is) => is(cause)) ? Effect.void : report(cause, input);

const report = (cause: unknown, input: ReportIn): Effect.Effect<void, never, ClientErrorReporter> =>
  ClientErrorReporter.use((reporter) => reporter.report(cause, input));

// attachedOr runs f, a read of an element or range that may have gone, and
// gives what it read, or fallback when it had gone. Any other failure is
// reported as operation, and gives fallback too.
export const attachedOr = <A>(
  operation: string,
  f: () => A,
  fallback: A,
): Effect.Effect<A, never, ClientErrorReporter> =>
  elementCall(f).pipe(
    Effect.catchTags({
      ElementGone: () => Effect.succeed(fallback),
      UnexpectedFailure: (failure) => Effect.as(report(failure.cause, { operation }), fallback),
    }),
  );

// readAttached is attachedOr for code that isn't Effect.
export function readAttached<A>(operation: string, f: () => A, fallback: A): A {
  return runSync(attachedOr(operation, f, fallback));
}

// allowed runs f, which uses a feature the browser may refuse, and says
// whether it ran. A refusal, or an element that has gone, is expected; any
// other failure is reported as operation.
export const allowed = (operation: string, f: () => void): Effect.Effect<boolean, never, ClientErrorReporter> =>
  refusableCall(f).pipe(
    Effect.as(true),
    Effect.catchTags({
      ElementGone: () => Effect.succeed(false),
      Refused: () => Effect.succeed(false),
      UnexpectedFailure: (failure) => Effect.as(report(failure.cause, { operation }), false),
    }),
  );

// whileAllowed is allowed for code that isn't Effect.
export function whileAllowed(operation: string, f: () => void): boolean {
  return runSync(allowed(operation, f));
}

// whileAttached is attached for code that isn't Effect.
export function whileAttached(operation: string, f: () => void): boolean {
  return runSync(attached(operation, f));
}

// reported is attempt for code that isn't Effect.
export function reported<A>(context: ReportContext & { readonly operation: string }, f: () => A, fallback: A): A {
  return runSync(attempt(context, f, fallback));
}

// cleanup runs f, which releases something -- a listener, a stream, an
// observer, an editor -- and reports its failure at warning.
export function cleanup(operation: string, f: () => void): void {
  reported({ operation, severity: "warning" }, f, undefined);
}

// reportRejection is a rejection handler that reports what it is given as
// context says.
export function reportRejection(context: ReportContext): (cause: unknown) => void {
  return reportUnless(context);
}

// reportUnless is a rejection handler that reports what it is given, unless
// one of expected says it is a failure the caller expects. A rejection runs
// its handler with none of the stack that made the promise, so the handler
// keeps the stack of the code that asked for it, for a failure with no stack
// of its own, such as a fetch's TypeError.
export function reportUnless(
  context: ReportContext,
  ...expected: ReadonlyArray<(cause: unknown) => boolean>
): (cause: unknown) => void {
  const handle = reportingUnless({ ...context, site: callSite(reportUnless) }, ...expected);
  return (cause) => runSync(handle(cause));
}

// reportUnlessAborted is a rejection handler that reports what it is given,
// unless it is a request that never completed.
export function reportUnlessAborted(context: ReportContext): (cause: unknown) => void {
  const handle = reportingUnless({ ...context, site: callSite(reportUnlessAborted) }, isAborted);
  return (cause) => runSync(handle(cause));
}
