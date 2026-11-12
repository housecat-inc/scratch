export function required<T>(value: T | null | undefined): T {
  if (value === null || value === undefined) throw new Error("Required markup or value is missing");
  return value;
}
