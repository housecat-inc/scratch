import * as Arr from "effect/Array";
import * as Effect from "effect/Effect";
import * as Result from "effect/Result";
import * as Schema from "effect/Schema";
import type * as SchemaAST from "effect/SchemaAST";
import type * as SchemaIssue from "effect/SchemaIssue";

import { SchemaMismatch } from "./failures";
import { ClientErrorReporter } from "./reporter";
import { runSync } from "./runtime";
import { toastAPI } from "./template_scripts/toast";

// How a script reads JSON the server sent: decoded with the Effect Schema
// generated from the Go type that sends it (ADR-0004), from
// app/ui/schemas/schemas.gen.ts or, in the shared UI, ui/schemas/schemas.gen.ts.
//
// JSON that doesn't match is reported as a SchemaMismatch, at error, and the
// caller gets the value as it came: a mismatch is a bug to fix, never a
// reason to stop the page, so the code after the decode can meet a shape its
// type rules out. The reporter sends each distinct mismatch once per page,
// and a user with developer mode on sees it as a toast too.

// WireContext is what a mismatch is reported with: the operation, as
// failures.ts names operations, and the endpoint the JSON came from, as its
// route names it, such as "/mail/:id/modify", never a URL with IDs in it.
export interface WireContext {
  readonly endpoint: string;
  readonly operation: string;
}

// Decodable is a Schema these decode with, for a script that takes one.
export type Decodable = Schema.ConstraintDecoder<unknown>;

// decoded is value decoded with schema, or value as it came when it doesn't
// match, once the mismatch is reported.
export const decoded = <S extends Decodable>(
  schema: S,
  context: WireContext,
  value: unknown,
): Effect.Effect<S["Type"], never, ClientErrorReporter> =>
  Effect.suspend(() => {
    const result = Schema.decodeUnknownResult(schema)(value, { errors: "all" });
    if (Result.isSuccess(result)) return Effect.succeed(result.success);
    return mismatched(schema, context, result.failure.issue).pipe(
      // oxlint-disable-next-line typescript/no-unsafe-type-assertion -- a mismatch gives the value as it came, which the page carries on with
      Effect.as(value as S["Type"]),
    );
  });

// mismatched reports the issue schema found in a value as a SchemaMismatch,
// as decoded does, for a caller that decoded the value itself, such as
// ui/dom/cookies.ts.
export const mismatched = (
  schema: Decodable,
  context: WireContext,
  issue: SchemaIssue.Issue,
): Effect.Effect<void, never, ClientErrorReporter> =>
  Effect.suspend(() => {
    const mismatch = new SchemaMismatch({ endpoint: context.endpoint, message: describe(schema.ast, issue) });
    toast(`${context.operation}: ${context.endpoint} ${mismatch.message}`);
    return ClientErrorReporter.use((reporter) =>
      reporter.report(mismatch, { fields: { endpoint: context.endpoint }, operation: context.operation })
    );
  });

// readDecoded is decoded for code that isn't Effect.
export function readDecoded<S extends Decodable>(schema: S, context: WireContext, value: unknown): S["Type"] {
  return runSync(decoded(schema, context, value));
}

const toasted = new Set<string>();

// toast shows message once per page to a user with developer mode on, which
// the document marks on <html>.
function toast(message: string): void {
  const developerMode = typeof document !== "undefined"
    && document.documentElement.hasAttribute("data-developer-mode");
  if (!developerMode || toasted.has(message)) return;
  toasted.add(message);
  toastAPI.show({ message });
}

// describe names each field that doesn't match by its path and the kind of
// issue, such as "counts.*.version[]: InvalidType", once however many of a
// map's entries or a list's items share it. Effect's own messages quote the
// value they rejected, so none of them is used.
function describe(ast: SchemaAST.AST, issue: SchemaIssue.Issue): string {
  return Arr.join(Arr.dedupe(Arr.map(leaves(issue, []), (leaf) => `${fieldPath(ast, leaf.path)}: ${leaf.kind}`)), "; ");
}

interface Leaf {
  readonly kind: string;
  readonly path: ReadonlyArray<PropertyKey>;
}

function leaves(issue: SchemaIssue.Issue, path: ReadonlyArray<PropertyKey>): ReadonlyArray<Leaf> {
  if (issue._tag === "Pointer") return leaves(issue.issue, [...path, ...issue.path]);
  if (issue._tag === "Filter" || issue._tag === "Encoding") return leaves(issue.issue, path);
  if (issue._tag === "Composite") return Arr.flatMap(issue.issues, (each) => leaves(each, path));
  if (issue._tag === "AnyOf") {
    // A value no member of a union could start on, such as a string where a
    // list or null goes, has no issue of its own.
    return Arr.isReadonlyArrayNonEmpty(issue.issues)
      ? Arr.flatMap(issue.issues, (each) => leaves(each, path))
      : [{ kind: "InvalidType", path }];
  }
  return [{ kind: issue._tag, path }];
}

// fieldPath is path as the Schema's field names, with a map's key, which is a
// label, a view or an address, as *, and an array's position as [].
function fieldPath(root: SchemaAST.AST, path: ReadonlyArray<PropertyKey>): string {
  let out = "";
  let node: SchemaAST.AST | undefined = root;
  for (const key of path) {
    if (node?._tag === "Union") {
      const want = typeof key === "number" ? "Arrays" : "Objects";
      node = node.types.find((member) => member._tag === want);
    }
    if (node?._tag === "Arrays") {
      out += "[]";
      node = typeof key === "number" ? node.elements[key] ?? node.rest[0] : undefined;
    } else if (node?._tag === "Objects") {
      const property: SchemaAST.PropertySignature | undefined = node.propertySignatures.find((p) => p.name === key);
      out += `${out ? "." : ""}${property ? String(key) : "*"}`;
      node = property ? property.type : node.indexSignatures[0]?.type;
    } else {
      out += `${out ? "." : ""}?`;
      node = undefined;
    }
  }
  return out || "(root)";
}
