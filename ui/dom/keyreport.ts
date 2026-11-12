// How the router reports a mistake without breaking the key it was handling.
// A handler, layer function or observer that throws is always reported, and
// the router carries on as though that one had declined. Strict mode --
// window.__housecatKeysStrict, which the browser tests set before any script
// runs -- also reports misuse that works now and breaks later: an owned
// registration with no id, two bindings tied on one element, an "always"
// binding taking typed text from a field it does not sit on, an observer that
// cancels a key.

declare global {
  interface Window {
    __housecatKeysStrict?: boolean;
  }
}

export function safely<T>(fn: () => T, fallback: T): T {
  try {
    return fn();
  } catch (error) {
    reportError(error);
    return fallback;
  }
}

export function misuse(message: string): void {
  if (window.__housecatKeysStrict === true) reportError(new Error("HousecatKeys: " + message));
}
