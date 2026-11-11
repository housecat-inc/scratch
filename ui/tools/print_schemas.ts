// Prints the module of Effect Schemas for the Go types a cmd/browserschemas
// command lists, from that command's JSON on stdin, to stdout. Each module's
// schemas package runs it from go:generate, naming the command:
//
//   go run ../cmd/browserschemas | vp env exec node ../tools/print_schemas.ts ui/cmd/browserschemas
import { readFileSync } from "node:fs";

import * as Schema from "effect/Schema";

import { decode } from "../dom/decode.ts";
import { schemaModule } from "./schema_module.ts";

const GoTypes = Schema.NonEmptyArray(
  Schema.Struct({ name: Schema.String, schema: Schema.Record(Schema.String, Schema.Unknown) }),
);

const source = process.argv[2];
if (!source) throw new Error("usage: print_schemas.ts <the cmd/browserschemas that lists the types>");
const types: unknown = JSON.parse(readFileSync(0, "utf8"));
process.stdout.write(schemaModule(source, decode(GoTypes)(types)));
