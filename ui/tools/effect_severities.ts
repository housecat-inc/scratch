// oxlint-disable effecttsgo/node-builtin-import -- The CLI and LSP transport require Node file descriptors, synchronous reads, raw Buffers, and process pipes.
import * as Arr from "effect/Array";
import * as Console from "effect/Console";
import * as Effect from "effect/Effect";
import * as Opt from "effect/Option";
import * as Rec from "effect/Record";
import * as Schema from "effect/Schema";
import * as Str from "effect/String";
import { decode } from "../dom/decode.ts";
import { required } from "../dom/required.ts";
import { indexOfFrom } from "../dom/search.ts";
const LintConfig = Schema.Struct({ rules: Schema.Record(Schema.String, Schema.Unknown) });
const Project = Schema.Struct({
  compilerOptions: Schema.Struct({
    plugins: Schema.Array(
      Schema.Struct({
        name: Schema.String,
        diagnosticSeverity: Schema.optionalKey(Schema.Record(Schema.String, Schema.String)),
      }),
    ),
  }),
});
const RuleDefinitions = Schema.Struct({
  definitions: Schema.Struct({
    effectLanguageServicePluginDiagnosticSeverityDefinition: Schema.Struct({
      properties: Schema.Record(Schema.String, Schema.Struct({ default: Schema.String })),
    }),
  }),
});
// Checks that ui/tsconfig.json's Effect plugin block mirrors the severities
// `vp lint` enforces for Effect's rules, which the root vite.config.ts sets.
// Editors' language server and `ui/tsc.sh -p` read the block; CI doesn't, so
// without this check what an editor shows could drift from what CI fails on.
// `make -C app lint-ts` runs it from the repo root, after patching Oxlint:
//
//   vp env exec node ui/tools/effect_severities.ts
//
// The language server gives a rule the block's severity, else Effect's default
// from @effect/tsgo's schema.json. Its errors and warnings mirror the lint's
// errors, its suggestions the lint's warnings, and off is off. Oxlint names a
// rule in kebab case, so names compare without dashes or case. Every project's
// tsconfig.json extends ui/tsconfig.json, and none sets a plugin block of its
// own, so this one block is the language server's for all of them.
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";

const here = new URL("../", import.meta.url);
const read = (path: string) => readFileSync(new URL(path, here), "utf8");
const key = (name: string) => Str.toLowerCase(Str.replaceAll("-", "")(name));
const fail = (message: string): never => {
  Effect.runSync(Console.error(`effect-severities: ${message}`));
  // oxlint-disable-next-line unicorn/no-process-exit -- The CLI must exit immediately with the compiler status and close its inherited stdio pipes.
  process.exit(1);
};

// tsconfig.json is JSON with comments and trailing commas, as TypeScript reads
// it: drop both, outside strings, and parse the rest as JSON.
const parseJsonc = (text: string): unknown => {
  let out = "";
  for (let i = 0; i < text.length; i++) {
    if (text[i] === '"') {
      let end = i + 1;
      while (end < text.length && text[end] !== '"') end += text[end] === "\\" ? 2 : 1;
      out += Str.slice(i, end + 1)(text);
      i = end;
    } else if (Str.startsWith("//", i)(text)) {
      const newline = indexOfFrom(text, "\n", i);
      i = newline < 0 ? text.length : newline - 1;
    } else if (Str.startsWith("/*", i)(text)) {
      i = indexOfFrom(text, "*/", i + 2) + 1;
    } else {
      out += text[i];
    }
  }
  return JSON.parse(Str.replaceAll(/,(\s*[}\]])/gu, "$1")(out));
};

let printed = "";
try {
  printed = execFileSync("vp", ["lint", "--print-config"], {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
} catch (err) {
  fail(`vp lint --print-config failed:\n${err instanceof Error ? err.message : String(err)}`);
}
const linted = new Map<string, [string, unknown]>();
for (const [name, value] of Rec.toEntries(decode(LintConfig)(JSON.parse(printed)).rules)) {
  if (Str.startsWith("effecttsgo/")(name)) {
    linted.set(key(Str.slice("effecttsgo/".length)(name)), [name, [value].flat()[0]]);
  }
}
const plugin = Opt.getOrUndefined(
  Arr.findFirst(
    decode(Project)(parseJsonc(read("tsconfig.json"))).compilerOptions.plugins,
    (p) => p.name === "@effect/language-service",
  ),
);
if (!plugin?.diagnosticSeverity) fail("Missing Effect language service diagnostics");
const rules = decode(RuleDefinitions)(JSON.parse(read("node_modules/@effect/tsgo/schema.json")))
  .definitions.effectLanguageServicePluginDiagnosticSeverityDefinition.properties;
const known = new Set<string>(Arr.map(Rec.keys(rules), key));
const mirror: Record<string, string | undefined> = {
  error: "deny",
  message: "warn",
  off: "allow",
  suggestion: "warn",
  warning: "deny",
};

const mismatches = [];
for (const name of Rec.keys(required(required(plugin).diagnosticSeverity))) {
  if (!(name in rules)) {
    mismatches.push(`${name}: ui/tsconfig.json names a rule @effect/tsgo doesn't have`);
  }
}
for (const [k, [name]] of linted) {
  if (!known.has(k)) {
    mismatches.push(`${name}: vite.config.ts names a rule @effect/tsgo doesn't have`);
  }
}
for (const [name, rule] of Rec.toEntries(rules)) {
  const server = required(required(plugin).diagnosticSeverity)[name] ?? rule.default;
  const lint = linted.get(key(name))?.[1] ?? "allow";
  if (mirror[server] !== lint) {
    mismatches.push(
      `${name}: the language server's ${server}, but vp lint's ${JSON.stringify(lint)}`,
    );
  }
}
if (mismatches.length > 0) {
  fail(
    `${
      Arr.join(mismatches, "\n")
    }\nui/tsconfig.json's Effect plugin block has to mirror the severities the root vite.config.ts gives vp lint. Change it to match.`,
  );
}
