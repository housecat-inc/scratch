import * as Option from "effect/Option";
import * as Str from "effect/String";

export function indexOfFrom(text: string, needle: string, start: number): number {
  const position = Math.min(text.length, Math.max(0, Math.trunc(start) || 0));
  return Option.getOrElse(
    Option.map(Str.indexOf(needle)(Str.slice(position)(text)), (offset) => offset + position),
    () => -1,
  );
}
