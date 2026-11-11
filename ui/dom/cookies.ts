// The cookies the browser keeps. Each holds JSON of a ui/wire type and is
// written and read only through that type's generated Schema (ADR-0004), so
// a cookie never carries a shape its readers, the server among them, don't
// expect. This module is the only browser code that touches document.cookie,
// in CookieJar.layer: TestCookiePolicy (ui/dom/cookie_policy_test.go) fails
// on any other.
//
// Effect's Cookies parses the jar and builds each Set-Cookie string, and
// Schema.fromJsonString carries a value to and from its JSON text. A read
// fails with CookieAbsent or Schema's SchemaError (text that is not JSON of
// the Schema's shape), a write with SchemaError or CookieError (a name or
// option a cookie can't have). cookieEntry and decodeCookie take a cookie
// string of their own; readCookie and writeCookie use the CookieJar service,
// the document's in the browser and an in-memory one in a test.

// Cookies is marked unstable in Effect 4, which the lint refuses elsewhere
// (effecttsgo/unstable-api-usage): its use stays in this one module, behind
// CookieError and the functions here, so a breaking change lands in one
// place.
/* oxlint-disable effecttsgo/unstable-api-usage */
import * as Context from "effect/Context";
import * as Data from "effect/Data";
import type * as Duration from "effect/Duration";
import * as Effect from "effect/Effect";
import { Cookies } from "effect/http";
import * as Layer from "effect/Layer";
import * as Opt from "effect/Option";
import * as Rec from "effect/Record";
import * as Ref from "effect/Ref";
import * as Schema from "effect/Schema";

// CookieAbsent is a cookie the jar doesn't have.
export class CookieAbsent extends Data.TaggedError("CookieAbsent")<{ readonly name: string }> {}

// CookieError is a cookie Effect's Cookies refuses to build: a name, value or
// option a cookie can't have.
export type CookieError = Cookies.CookiesError;

// CookieSchema is a ui/wire type's generated Schema, which a cookie's value
// is encoded with and decoded by.
export type CookieSchema<T> = Schema.Codec<T, unknown>;

// CookieJar is where cookies are kept: read is every cookie as a Cookie
// header, write takes a Set-Cookie string.
export class CookieJar extends Context.Service<CookieJar, {
  readonly read: Effect.Effect<string>;
  readonly write: (entry: string) => Effect.Effect<void>;
}>()("@housecat/ui/dom/CookieJar") {
  // layer is the document's jar. A write lands in document.cookie
  // synchronously, so the very next request carries it.
  static readonly layer = Layer.succeed(
    CookieJar,
    CookieJar.of({
      read: Effect.sync(() => document.cookie),
      write: (entry) =>
        Effect.sync(() => {
          // oxlint-disable-next-line unicorn/no-document-cookie -- The server must see this cookie on the next immediate navigation; Cookie Store writes are asynchronous.
          document.cookie = entry;
        }),
    }),
  );

  // layerTest is a jar in memory, which keeps each cookie written as a
  // browser would, the latest under each name.
  static readonly layerTest = Layer.effect(
    CookieJar,
    Effect.map(Ref.make(Cookies.empty), (jar) =>
      CookieJar.of({
        read: Effect.map(Ref.get(jar), Cookies.toCookieHeader),
        write: (entry) => Ref.update(jar, (cookies) => Cookies.merge(cookies, Cookies.fromSetCookie(entry))),
      })),
  );
}

// cookieEntry is the Set-Cookie string that saves value, encoded with schema
// as JSON, under name for the whole site for maxAge.
export const cookieEntry = Effect.fnUntraced(function*<T>(
  schema: CookieSchema<T>,
  name: string,
  value: T,
  maxAge: Duration.Input,
): Effect.fn.Return<string, Schema.SchemaError | CookieError> {
  const json = yield* Schema.encodeEffect(Schema.fromJsonString(schema))(value);
  const cookie = yield* Effect.fromResult(Cookies.makeCookie(name, json, { maxAge, path: "/", sameSite: "lax" }));
  return Cookies.serializeCookie(cookie);
});

// decodeCookie is name's value in cookies, a Cookie header, decoded with
// schema from its JSON.
export const decodeCookie = <T>(
  schema: CookieSchema<T>,
  cookies: string,
  name: string,
): Effect.Effect<T, CookieAbsent | Schema.SchemaError> =>
  Opt.match(Rec.get(Cookies.parseHeader(cookies), name), {
    onNone: () => Effect.fail(new CookieAbsent({ name })),
    onSome: (text) => Schema.decodeUnknownEffect(Schema.fromJsonString(schema))(text),
  });

// readCookie is name's value in the jar, decoded with schema.
export const readCookie = Effect.fnUntraced(function*<T>(
  schema: CookieSchema<T>,
  name: string,
): Effect.fn.Return<T, CookieAbsent | Schema.SchemaError, CookieJar> {
  const jar = yield* CookieJar;
  return yield* decodeCookie(schema, yield* jar.read, name);
});

// writeCookie saves value under name in the jar, encoded with schema, for
// maxAge.
export const writeCookie = Effect.fnUntraced(function*<T>(
  schema: CookieSchema<T>,
  name: string,
  value: T,
  maxAge: Duration.Input,
): Effect.fn.Return<void, Schema.SchemaError | CookieError, CookieJar> {
  const jar = yield* CookieJar;
  yield* jar.write(yield* cookieEntry(schema, name, value, maxAge));
});
