export type OptimisticSnapshot<Value, Version> = {
  value: Value;
  version: Version;
};

export function createOptimisticState<Value, Version>(
  initial: OptimisticSnapshot<Value, Version>,
  compare: (a: Version, b: Version) => number,
) {
  let snapshot = initial;
  const pending = new Map<string, (value: Value) => Value>();

  return {
    base(): Value {
      return snapshot.value;
    },
    get(): Value {
      let value = snapshot.value;
      for (const reduce of pending.values()) value = reduce(value);
      return value;
    },
    set(id: string, reduce: (value: Value) => Value): void {
      pending.set(id, reduce);
    },
    remove(id: string): void {
      pending.delete(id);
    },
    receive(next: OptimisticSnapshot<Value, Version>, included: Iterable<string>): boolean {
      for (const id of included) pending.delete(id);
      if (compare(next.version, snapshot.version) < 0) return false;
      snapshot = next;
      return true;
    },
    pending(): string[] {
      return [...pending.keys()];
    },
  };
}
