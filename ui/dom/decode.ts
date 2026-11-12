import * as Result from "effect/Result";
import * as Schema from "effect/Schema";

export function decode<S extends Schema.ConstraintDecoder<unknown>>(
  schema: S,
): (value: unknown) => S["Type"] {
  const parse = Schema.decodeUnknownResult(schema);
  return (value) => Result.getOrThrowWith(parse(value), (error) => error);
}
