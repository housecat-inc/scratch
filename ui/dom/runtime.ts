import * as Cause from "effect/Cause";
import * as Data from "effect/Data";
import * as Effect from "effect/Effect";
import * as Exit from "effect/Exit";
import type * as Fiber from "effect/Fiber";
import * as Layer from "effect/Layer";
import * as Logger from "effect/Logger";
import * as ManagedRuntime from "effect/ManagedRuntime";

import { type ReportContext, type ReportIn, type Source } from "./clienterror";
import { ClientErrorReporter } from "./reporter";

// The shared Effect runtime: browser code runs its Effects through these
// functions, never a bare Effect.run*, so that what goes wrong in them is
// reported. The runtime provides the client error reporter, a logger that
// reports each log at Error level or above, and reports each defect a fiber
// it runs dies of. A typed failure stays the caller's: it rejects runPromise,
// or ends the fiber runFork returns, for the caller to handle by tag.

// errorLogger reports what is logged at Error or Fatal, beside the default
// logger, which keeps writing it to the console.
const errorLogger = Effect.gen(function*() {
  const reporter = yield* ClientErrorReporter;
  const services = yield* Effect.context();
  return Logger.make(({ cause, logLevel, message }) => {
    if (logLevel !== "Error" && logLevel !== "Fatal") return;
    const failure = cause.reasons.length ? Cause.squash(cause) : new LoggedError({ message: messageText(message) });
    Effect.runSyncWith(services)(reporter.report(failure, { source: "log" }));
  });
});

// A LoggedError is a log at Error level that came with no failure of its own.
class LoggedError extends Data.TaggedError("LoggedError")<{ readonly message: string }> {}

function messageText(message: unknown): string {
  const parts = Array.isArray(message) ? message : [message];
  return parts.filter((part) => typeof part === "string").join(" ");
}

// runtimeLayer is the runtime's layer with reporter's reporter: the live one
// in the browser, layerTest in a test.
export function runtimeLayer<ROut, E>(
  reporter: Layer.Layer<ROut | ClientErrorReporter, E>,
): Layer.Layer<ROut | ClientErrorReporter, E> {
  return Logger.layer([errorLogger], { mergeWithExisting: true }).pipe(Layer.provideMerge(reporter));
}

export const runtime = ManagedRuntime.make(runtimeLayer(ClientErrorReporter.layer));

type Services = ManagedRuntime.ManagedRuntime.Services<typeof runtime>;

// reportingDefects reports each defect effect dies of, and dies of it still.
export function reportingDefects<A, E, R>(
  effect: Effect.Effect<A, E, R>,
): Effect.Effect<A, E, R | ClientErrorReporter> {
  return Effect.tapDefect(
    effect,
    (defect) => ClientErrorReporter.use((reporter) => reporter.report(defect, { source: "defect" })),
  );
}

export function runFork<A, E>(effect: Effect.Effect<A, E, Services>, options?: Effect.RunOptions): Fiber.Fiber<A, E> {
  return runtime.runFork(reportingDefects(effect), options);
}

export function runPromise<A, E>(effect: Effect.Effect<A, E, Services>, options?: Effect.RunOptions): Promise<A> {
  return runtime.runPromise(reportingDefects(effect), options);
}

export function runSync<A, E>(effect: Effect.Effect<A, E, Services>): A {
  return runtime.runSync(reportingDefects(effect));
}

// reportError reports a failure that code caught and did not expect, with
// what context says about it, and returns at once. A caller keeps whatever
// fallback it had; reportError never throws.
export function reportError(failure: unknown, context: ReportContext = {}): void {
  report(failure, { ...context, site: callSite(reportError) });
}

// reportUncaught reports a failure a global handler caught.
export function reportUncaught(failure: unknown, source: Source, site = ""): void {
  report(failure, { site, source });
}

function report(failure: unknown, input: ReportIn): void {
  const exit = runtime.runSyncExit(ClientErrorReporter.use((reporter) => reporter.report(failure, input)));
  if (Exit.isFailure(exit)) console.error("Housecat could not report an error", failure, Cause.pretty(exit.cause));
}

// callSite is the stack of the call that reported a failure, from below
// reported, for a failure whose own stack names none of our bundles, such as
// a fetch's TypeError.
export function callSite(reported: (...args: never[]) => unknown): string {
  const holder: { stack?: string } = {};
  // Older engines have no Error.captureStackTrace, which its types leave out.
  if (typeof Error.captureStackTrace === "function") {
    Error.captureStackTrace(holder, reported);
    return holder.stack ?? "";
  }
  return new Error().stack ?? "";
}
