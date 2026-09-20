// Test-only runtime schema stand-in. Pi validates these schemas in production.
export const Type = {
  Object: (value: unknown) => value,
  Union: (value: unknown) => value,
  Literal: (value: unknown) => value,
  String: () => ({ type: "string" }),
  Number: () => ({ type: "number" }),
  Array: (value: unknown) => value,
  Optional: (value: unknown) => value,
};
