import * as Clock from "effect/Clock";
import * as Context from "effect/Context";
import type * as Duration from "effect/Duration";
import * as Effect from "effect/Effect";
import * as Layer from "effect/Layer";
import * as Ref from "effect/Ref";
import type * as Schema from "effect/Schema";

import { type ClientErrorReport, describeFailure, type PageIn, type ReportIn, reportKey } from "./clienterror";

// The client error reporter: the one way browser code reports a failure it
// did not expect. It sends reports to the installation's own server, which
// records them in Sentry through trace, so no Sentry key is in the page and
// the server's scrubbing applies. Code reports through the shared runtime
// (runtime.ts), which provides the live layer; a Vitest suite provides
// layerTest, which records reports in memory.

// endpoint is where the server takes reports, as app/srv's clientErrorsPath
// names it.
export const endpoint = "/api/client-errors";

// ClientErrorBatch is the body the reporter posts to endpoint, which the
// server decodes as app/wire's ClientErrorsIn.
export interface ClientErrorBatch {
  readonly reports: ReadonlyArray<ClientErrorReport>;
}

// typeCheckBatchesAgainst has the compiler check the batches the reporter
// sends against the Go type the server decodes them into, whose Schema a ui
// script can't import (ADR-0004). The app's entry calls it with that Schema,
// and the call compiles only when every batch fits the Schema's type with no
// key it lacks: contract.Decode rejects a batch that doesn't. It decodes
// nothing and does nothing at run time, so a page without the app's bundle
// reports just the same.
export function typeCheckBatchesAgainst<S extends Schema.Top>(_schema: S & BatchFits<S["Type"]>): void {
  // The check is the call's type.
}

// BatchFits is unknown when every ClientErrorBatch is a T with no key T
// lacks, and otherwise names what doesn't fit.
type BatchFits<T> = [ClientErrorBatch] extends [T] ? NoneLacked<KeysLacked<ClientErrorBatch, T>>
  : { readonly batchIsNot: T };

type NoneLacked<Keys> = [Keys] extends [never] ? unknown : { readonly keysTheServerLacks: Keys };

// KeysLacked is each key of Sent, at any depth, that Target lacks.
type KeysLacked<Sent, Target> = Sent extends ReadonlyArray<infer Item>
  ? KeysLacked<Item, NonNullable<Target> extends ReadonlyArray<infer Want> ? Want : never>
  : Sent extends object ? {
      [K in keyof Sent]-?: K extends keyof NonNullable<Target> ? KeysLacked<Sent[K], NonNullable<Target>[K]> : K;
    }[keyof Sent]
  : never;

// A Page is what the reporter reads of the page it reports from, and how it
// sends. browserPage is the real one; a test hands makeReporter its own.
export interface Page {
  readonly dev: boolean;
  // log writes a report in full to the console, which dev mode does for each
  // one, whether or not it is sent.
  readonly log: (report: ClientErrorReport, failure: unknown) => void;
  // onHide calls flush as the page goes away or out of sight.
  readonly onHide: (flush: () => void) => void;
  readonly origin: string;
  readonly path: () => string;
  readonly requestId: string;
  // send posts body to the endpoint. hiding says the page is going away, so
  // the request has to leave now and outlive it. A send that fails is
  // dropped: the reporter has nowhere else to say so.
  readonly send: (body: string, hiding: boolean) => Effect.Effect<void>;
}

export interface Limits {
  // batch is how many reports go in one request.
  readonly batch: number;
  // delay is how long a report waits for others to go with it.
  readonly delay: Duration.Input;
  readonly perMinute: number;
  readonly perPage: number;
  // seen is how many distinct reports a page remembers, to send each once.
  readonly seen: number;
}

export const defaultLimits: Limits = { batch: 10, delay: "2 seconds", perMinute: 10, perPage: 50, seen: 200 };

// ClientErrorReports is what layerTest's reporter has recorded, in order.
export class ClientErrorReports
  extends Context.Service<ClientErrorReports, Ref.Ref<ReadonlyArray<ClientErrorReport>>>()(
    "housecat/ui/dom/reporter/ClientErrorReports",
  )
{
  static readonly layer = Layer.effect(ClientErrorReports, Ref.make<ReadonlyArray<ClientErrorReport>>([]));
}

// testPage is the page layerTest's reports say they came from.
export const testPage: Omit<PageIn, "time"> = { origin: "https://housecat.test", path: "/test", requestId: "" };

export class ClientErrorReporter extends Context.Service<ClientErrorReporter, {
  // report reports failure with what input adds about it. It never fails and
  // returns at once; the report is sent a moment later, with any that follow
  // it.
  report: (failure: unknown, input?: ReportIn) => Effect.Effect<void>;
  // flush sends every report still waiting.
  readonly flush: Effect.Effect<void>;
}>()("housecat/ui/dom/reporter/ClientErrorReporter") {
  static readonly layer = Layer.effect(ClientErrorReporter, Effect.suspend(() => makeReporter(browserPage())));

  static readonly layerTest = Layer.effect(
    ClientErrorReporter,
    Effect.gen(function*() {
      const reports = yield* ClientErrorReports;
      return ClientErrorReporter.of({
        flush: Effect.void,
        report: (failure, input = {}) =>
          Effect.gen(function*() {
            const time = yield* Clock.currentTimeMillis;
            const report = describeFailure(failure, input, { ...testPage, time });
            yield* Ref.update(reports, (all) => [...all, report]);
          }),
      });
    }),
  ).pipe(Layer.provideMerge(ClientErrorReports.layer));
}

// makeReporter is the live reporter for page. It sends each distinct failure
// once per page, at most limits.perMinute a minute and limits.perPage in all,
// so a page caught in an error loop sends a bounded number. A report waits
// limits.delay for others to go with it, and every report still waiting is
// sent as the page hides.
export const makeReporter = Effect.fnUntraced(function*(page: Page, limits: Limits = defaultLimits) {
  const services = yield* Effect.context();
  const seen = new Set<string>();
  const waiting: ClientErrorReport[] = [];
  let scheduled = false;
  let sent = 0;
  let minute = { count: 0, start: 0 };

  const sendWaiting = (hiding: boolean) =>
    Effect.suspend(() => {
      scheduled = false;
      const batches: ClientErrorReport[][] = [];
      while (waiting.length) batches.push(waiting.splice(0, limits.batch));
      return Effect.forEach(
        batches,
        (reports) => page.send(JSON.stringify({ reports } satisfies ClientErrorBatch), hiding),
        { discard: true },
      );
    });
  const flush = sendWaiting(false);
  page.onHide(() => Effect.runForkWith(services)(sendWaiting(true)));

  const report = (failure: unknown, input: ReportIn = {}) =>
    Effect.gen(function*() {
      const time = yield* Clock.currentTimeMillis;
      const built = describeFailure(failure, input, {
        origin: page.origin,
        path: page.path(),
        requestId: page.requestId,
        time,
      });
      if (page.dev) page.log(built, failure);
      const key = reportKey(built);
      if (seen.has(key)) return;
      if (time - minute.start >= 60_000) minute = { count: 0, start: time };
      if (sent >= limits.perPage || minute.count >= limits.perMinute) return;
      if (seen.size < limits.seen) seen.add(key);
      sent++;
      minute.count++;
      waiting.push(built);
      if (scheduled) return;
      scheduled = true;
      yield* Effect.forkDetach(Effect.andThen(Effect.sleep(limits.delay), flush));
    });

  return ClientErrorReporter.of({ flush, report });
});

// browserPage is the page this script runs in. Its request ID and dev mode
// come from the meta tags the app's document layout writes. Outside a
// browser, as in a Vitest suite of a module that reports, there is no page,
// and nothing is sent.
function browserPage(): Page {
  if (typeof document === "undefined") return absentPage;
  const meta = (name: string) => document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)?.content ?? "";
  return {
    dev: meta("hc-dev") === "true",
    log: (report, failure) => console.warn("Housecat client error report", report, failure),
    onHide: (flush) => {
      window.addEventListener("pagehide", flush);
      document.addEventListener("visibilitychange", () => {
        if (document.visibilityState === "hidden") flush();
      });
    },
    origin: location.origin,
    path: () => location.pathname,
    requestId: meta("hc-request-id"),
    send: (
      body,
      hiding,
    ) => (hiding
      ? Effect.suspend(() => (navigator.sendBeacon(endpoint, body) ? Effect.void : post(body, true)))
      : post(body, false)),
  };
}

const absentPage: Page = {
  dev: false,
  log: () => undefined,
  onHide: () => undefined,
  origin: "",
  path: () => "/",
  requestId: "",
  send: () => Effect.void,
};

// post sends body as a request, which keepalive lets outlive the page. Its
// failure, offline or a page gone too soon, is dropped.
function post(body: string, keepalive: boolean): Effect.Effect<void> {
  return Effect.tryPromise(() =>
    fetch(endpoint, {
      body,
      credentials: "same-origin",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      keepalive,
      method: "POST",
    })
  ).pipe(Effect.ignore);
}
