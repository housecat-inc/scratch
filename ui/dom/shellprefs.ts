// The app shell's panel preferences: the cookies the server reads to draw
// the next page the way the user left it (srv's shellStateFor). ShellPrefs is
// the one writer of those cookies. Their names come from the server, in the
// shell's shell-cookies script (a ui/wire ShellCookies, which
// hclayouts.ShellCookies fills), so a name is spelled once, in Go; the
// encoding and the attributes are here and in ./cookies, once.
//
// Each cookie holds JSON of a ui/wire type, written and read through
// ./cookies with that type's generated Schema, and the server decodes it
// with the same type, so neither side reads a shape the other did not write.
// A value that does not decode is a bug, reported, and reads as nothing
// saved. The cookies were renamed when their values became JSON, so one
// saved in the older query-string form is simply not read.
//
// A preference is a cookie rather than storage because the server has to see
// it on the very next request, which may already be on its way: a write
// lands in document.cookie synchronously, ahead of it. A page without the
// script (a story, a component test) has no shell to remember, so reads
// find nothing and writes are dropped.
//
// Scripts that aren't Effect call navSections, saveNavSections and
// saveChatLast, which run the page's one ShellPrefs.

import * as Context from "effect/Context";
import * as Effect from "effect/Effect";
import * as Layer from "effect/Layer";
import * as ManagedRuntime from "effect/ManagedRuntime";
import * as Schema from "effect/Schema";

import * as Wire from "../schemas/schemas.gen";
import { scrubPath } from "./clienterror";
import { type CookieError, CookieJar, type CookieSchema, readCookie, writeCookie } from "./cookies";
import { ClientErrorReporter } from "./reporter";
import { reportingDefects, runtime } from "./runtime";
import { mismatched } from "./wire";

// A preference outlives any one visit.
const maxAge = "365 days";

// ShellPage is what ShellPrefs is made for: the cookies' names, null on a
// page without the shell-cookies script, and the page's path, which reports
// name as their endpoint.
export interface ShellPage {
  readonly names: Wire.ShellCookies | null;
  readonly path: string;
}

// makeShellPrefs is ShellPrefs for page, over the jar and the reporter it
// is made with.
const makeShellPrefs = Effect.fnUntraced(function*(page: ShellPage) {
  const services = yield* Effect.context<CookieJar | ClientErrorReporter>();
  const endpoint = scrubPath(page.path);
  const reportMismatch = <T>(schema: CookieSchema<T>, operation: string) => (error: Schema.SchemaError) =>
    mismatched(schema, { endpoint, operation }, error.issue);
  const reportCookie = (operation: string) => (error: CookieError) =>
    ClientErrorReporter.use((reporter) => reporter.report(error, { operation }));
  const { names } = page;

  const navSections: Effect.Effect<Record<string, boolean>> = names === null
    ? Effect.succeed({})
    : readCookie(Wire.NavSections, names.nav_sections).pipe(
      Effect.map((sections) => ({ ...sections })),
      Effect.catchTags({
        CookieAbsent: () => Effect.succeed({}),
        SchemaError: (error) => reportMismatch(Wire.NavSections, "ShellPrefs.navSections")(error).pipe(Effect.as({})),
      }),
      Effect.provideContext(services),
    );

  const saveNavSections = Effect.fn("ShellPrefs.saveNavSections")(
    function*(sections: Record<string, boolean>) {
      if (names !== null) yield* writeCookie(Wire.NavSections, names.nav_sections, sections, maxAge);
    },
    Effect.catchTags({
      CookiesError: reportCookie("ShellPrefs.saveNavSections"),
      SchemaError: reportMismatch(Wire.NavSections, "ShellPrefs.saveNavSections"),
    }),
    Effect.provideContext(services),
  );

  const saveChatLast = Effect.fn("ShellPrefs.saveChatLast")(
    function*(id: string) {
      if (names !== null) yield* writeCookie(Wire.ChatLast, names.chat_last, { id }, maxAge);
    },
    Effect.catchTags({
      CookiesError: reportCookie("ShellPrefs.saveChatLast"),
      SchemaError: reportMismatch(Wire.ChatLast, "ShellPrefs.saveChatLast"),
    }),
    Effect.provideContext(services),
  );

  return ShellPrefs.of({ navSections, saveChatLast, saveNavSections });
});

// documentPage is the page this document is: the names from its
// shell-cookies script, and its path. A script that doesn't decode is a bug,
// reported, and leaves the shell nothing to remember.
const documentPage: Effect.Effect<ShellPage, never, ClientErrorReporter> = Effect.gen(function*() {
  const path = location.pathname;
  const text = document.getElementById("shell-cookies")?.textContent;
  if (text === undefined) return { names: null, path };
  const schema = Schema.fromJsonString(Wire.ShellCookies);
  const names = yield* Schema.decodeUnknownEffect(schema)(text).pipe(
    Effect.catchTag(
      "SchemaError",
      (error) =>
        mismatched(schema, { endpoint: scrubPath(path), operation: "ShellPrefs.names" }, error.issue).pipe(
          Effect.as(null),
        ),
    ),
  );
  return { names, path };
});

// ShellPrefs reads and saves the shell's panel preferences. A read never
// fails: nothing saved reads as nothing chosen. A save that can't be made is
// a bug, reported, and saves nothing.
export class ShellPrefs extends Context.Service<ShellPrefs, {
  // navSections is the sidebar sections the user opened (true) or closed
  // (false), as saved.
  readonly navSections: Effect.Effect<Record<string, boolean>>;
  // saveNavSections saves the sidebar sections the user opened or closed.
  readonly saveNavSections: (sections: Record<string, boolean>) => Effect.Effect<void>;
  // saveChatLast saves the chat the chat panel last showed.
  readonly saveChatLast: (id: string) => Effect.Effect<void>;
}>()("@housecat/ui/dom/ShellPrefs") {
  // layer is the document's ShellPrefs, with the names its shell-cookies
  // script gives, read once as the layer is built.
  static readonly layer = Layer.effect(ShellPrefs, Effect.flatMap(documentPage, makeShellPrefs));

  // layerFor is ShellPrefs for a page of a test's own.
  static readonly layerFor = (page: ShellPage) => Layer.effect(ShellPrefs, makeShellPrefs(page));
}

// The scripts' ShellPrefs, built once a page on the shared runtime's memo map,
// so it reports through the page's one reporter.
const live = ManagedRuntime.make(
  ShellPrefs.layer.pipe(Layer.provide(CookieJar.layer), Layer.provideMerge(ClientErrorReporter.layer)),
  { memoMap: runtime.memoMap },
);

function run<A>(f: (prefs: ShellPrefs["Service"]) => Effect.Effect<A>): A {
  return live.runSync(reportingDefects(ShellPrefs.use(f)));
}

// navSections is ShellPrefs' navSections for a script that isn't Effect.
export function navSections(): Record<string, boolean> {
  return run((prefs) => prefs.navSections);
}

// saveNavSections is ShellPrefs' saveNavSections for a script that isn't
// Effect.
export function saveNavSections(sections: Record<string, boolean>): void {
  run((prefs) => prefs.saveNavSections(sections));
}

// saveChatLast is ShellPrefs' saveChatLast for a script that isn't Effect.
export function saveChatLast(id: string): void {
  run((prefs) => prefs.saveChatLast(id));
}
