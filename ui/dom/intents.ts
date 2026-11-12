import * as Arr from "effect/Array";
import * as Effect from "effect/Effect";
import * as Fiber from "effect/Fiber";
import * as Option from "effect/Option";
import * as Semaphore from "effect/Semaphore";
import { createOptimisticState } from "./optimistic";

export { createOptimisticState } from "./optimistic";

// An intent store holds, per scope and key, what the user wants against
// what the server last confirmed, and saves the difference: one request at a
// time per scope, in the order changes were made, with the body worked out
// when the request's turn comes so quick changes fold into it. A failure
// puts back what the server has; a page being left flushes what is still
// pending. It knows no DOM and nothing about what a key means: an adapter
// supplies fold, send and what to do with an answer. Names follow TanStack
// Query (scope, optimistic set, rollback on error) so they read familiarly.

// V is compared with === throughout, so it must be a primitive (a boolean, a
// string, a number): an object value would read as pending for ever.
export type Change<V> = { key: string; value: V };

export type IntentOptions<V, Body, Result> = {
  // Pending changes for one scope -> one request body; null means nothing to send.
  fold: (changes: Change<V>[]) => Body | null;
  // Sends one request. keepalive is true for a leave-time flush.
  send: (scope: string, body: Body, keepalive: boolean) => Promise<Result>;
  // Maps sent values to the authoritative values acknowledged by the server.
  confirm?: (scope: string, changes: Change<V>[], result: Result) => Change<V>[];
  // The answer to the latest request the scope sent; earlier answers are not
  // reported. A flush is not counted, so the answer of the request before
  // it still applies; an answer may be undefined.
  answered?: (scope: string, result: Result | undefined) => void;
  // Once per failed request, with the changes that request carried.
  failed?: (scope: string, changes: Change<V>[]) => void;
};

export type Render<V> = (key: string, value: V) => void;

export interface IntentStore<V> {
  // What the user wants now. Subscribers redraw at once; the save follows in turn.
  set: (scope: string, key: string, value: V) => void;
  // What the server rendered. Ignored for a key with a change pending or in
  // flight; returns whether it was taken, so state that follows the key can
  // follow it only where the store did.
  seed: (scope: string, key: string, confirmed: V) => boolean;
  // The wanted value, else the confirmed one, else undefined.
  get: (scope: string, key: string) => V | undefined;
  // key null means any key in the scope. Rendered at once with the current
  // value if one is known, and whenever it changes, until signal aborts.
  subscribe: (scope: string, key: string | null, render: Render<V>, signal: AbortSignal) => void;
  // Resolves once the scope has nothing pending, in flight or queued through run.
  settled: (scope: string) => Promise<void>;
  // Runs fn in the scope's serial lane, after everything queued before it.
  run: <T>(scope: string, fn: () => Promise<T>) => Promise<T>;
  // Sends every scope's pending changes now, with keepalive, for a page being left.
  flush: () => void;
}

type Box<V> = { value: V };

// A key's slot: wanted is the user's latest ask, confirmed the server's last
// known state, and carrying the values requests in flight are taking to the
// server, oldest first. wanted is cleared once the server has it and nothing
// is in flight, so a defined wanted means pending or in flight.
type Slot<V> = { wanted?: Box<V>; confirmed?: Box<V>; carrying: V[] };

type Sub<V> = { key: string | null; render: Render<V> };

type Scope<V> = {
  readonly id: string;
  keys: Map<string, Slot<V>>;
  // Whether a view ever subscribed: eviction is for a scope whose views have
  // gone, not for one that was only ever driven from behind.
  subscribed: boolean;
  // The lane runs sends and run() work one after another.
  lane: Semaphore.Semaphore;
  // A send is scheduled from being appended to the lane until it has folded.
  scheduled: boolean;
  // Everything queued or in flight for the scope, flushes included, for settled.
  active: Set<Fiber.Fiber<unknown, unknown>>;
  requests: number;
  subs: Set<Sub<V>>;
};

// IntentRuntime is the Effect the store uses, so a test can hand it its own.
// The default names each function rather than passing the modules whole, so
// a bundle keeps only these of Effect.
export type IntentRuntime = {
  Effect: Pick<
    typeof Effect,
    | "ensuring"
    | "gen"
    | "matchEffect"
    | "runFork"
    | "runPromise"
    | "suspend"
    | "sync"
    | "tryPromise"
    | "void"
    | "yieldNow"
  >;
  Fiber: Pick<typeof Fiber, "awaitAll" | "join">;
  Semaphore: Pick<typeof Semaphore, "makeUnsafe" | "withPermit">;
};

const effect: IntentRuntime = {
  Effect: {
    ensuring: Effect.ensuring,
    gen: Effect.gen,
    matchEffect: Effect.matchEffect,
    runFork: Effect.runFork,
    runPromise: Effect.runPromise,
    suspend: Effect.suspend,
    sync: Effect.sync,
    tryPromise: Effect.tryPromise,
    void: Effect.void,
    yieldNow: Effect.yieldNow,
  },
  Fiber: { awaitAll: Fiber.awaitAll, join: Fiber.join },
  Semaphore: { makeUnsafe: Semaphore.makeUnsafe, withPermit: Semaphore.withPermit },
};

export function createIntentStore<V, Body, Result>(
  options: IntentOptions<V, Body, Result>,
  runtime: IntentRuntime = effect,
): IntentStore<V> {
  const { Effect, Fiber, Semaphore } = runtime;
  const scopes = new Map<string, Scope<V>>();

  function scopeOf(id: string): Scope<V> {
    let scope = scopes.get(id);
    if (!scope) {
      scope = {
        active: new Set(),
        id,
        keys: new Map(),
        lane: Semaphore.makeUnsafe(1),
        requests: 0,
        scheduled: false,
        subs: new Set(),
        subscribed: false,
      };
      scopes.set(id, scope);
    }
    return scope;
  }

  function slotOf(scope: Scope<V>, key: string): Slot<V> {
    let slot = scope.keys.get(key);
    if (!slot) {
      slot = { carrying: [] };
      scope.keys.set(key, slot);
    }
    return slot;
  }

  // expected is the state the server will have once what is in flight lands:
  // the newest value carried, else what it last confirmed.
  function expected(slot: Slot<V>): Box<V> | undefined {
    // A carried value may itself be undefined, so the newest is an Option.
    return Option.match(Arr.last(slot.carrying), {
      onNone: () => slot.confirmed,
      onSome: (value) => ({ value }),
    });
  }

  function pending(slot: Slot<V>): boolean {
    if (slot.wanted === undefined) return false;
    const ahead = expected(slot);
    return ahead === undefined || ahead.value !== slot.wanted.value;
  }

  function anyPending(scope: Scope<V>): boolean {
    for (const slot of scope.keys.values()) if (pending(slot)) return true;
    return false;
  }

  function current(slot: Slot<V> | undefined): Box<V> | undefined {
    return slot?.wanted ?? slot?.confirmed;
  }

  function notify(scope: Scope<V>, key: string, value: V): void {
    scope.subs.forEach((sub) => {
      if (sub.key === null || sub.key === key) sub.render(key, value);
    });
  }

  function start<A, E>(scope: Scope<V>, task: Effect.Effect<A, E>): Fiber.Fiber<A, E> {
    const fiber = Effect.runFork(task);
    scope.active.add(fiber);
    fiber.addObserver(() => {
      scope.active.delete(fiber);
      evict(scope);
    });
    return fiber;
  }

  // evict drops a scope whose views have all gone once nothing is queued or
  // in flight and no wish is outstanding. Its confirmed values go with it;
  // the next view to bind seeds them again from what the server drew. Only
  // the end of a request or of a subscription asks, so a seed followed by a
  // subscribe in the same task is never caught between the two, and a scope
  // no view ever watched is left as it is.
  function evict(scope: Scope<V>): void {
    if (!scope.subscribed || scope.subs.size || scope.active.size || scope.scheduled) return;
    for (const slot of scope.keys.values()) if (slot.wanted !== undefined || slot.carrying.length) return;
    if (scopes.get(scope.id) === scope) scopes.delete(scope.id);
  }

  function enqueue<A, E>(scope: Scope<V>, task: Effect.Effect<A, E>): Fiber.Fiber<A, E> {
    return start(
      scope,
      Effect.gen(function*() {
        yield* Effect.yieldNow;
        return yield* Semaphore.withPermit(scope.lane, task);
      }),
    );
  }

  // sendable is the scope's pending changes. A key a flush has already sent is
  // measured against that carried value, so it is not pending and not resent.
  function sendable(scope: Scope<V>): Change<V>[] {
    const changes: Change<V>[] = [];
    scope.keys.forEach((slot, key) => {
      if (pending(slot)) changes.push({ key, value: slot.wanted!.value });
    });
    return changes;
  }

  function schedule(id: string, scope: Scope<V>): void {
    if (scope.scheduled) return;
    scope.scheduled = true;
    enqueue(
      scope,
      Effect.suspend(() => {
        scope.scheduled = false;
        const changes = sendable(scope);
        if (!changes.length) return Effect.void;
        const body = options.fold(changes);
        return body === null ? Effect.void : dispatch(id, scope, changes, body, false);
      }),
    );
  }

  function dispatch(
    id: string,
    scope: Scope<V>,
    changes: Change<V>[],
    body: Body,
    keepalive: boolean,
  ): Effect.Effect<void> {
    return Effect.suspend(() => {
      changes.forEach((c) => slotOf(scope, c.key).carrying.push(c.value));
      // A leave-time flush must not hide the answer of an earlier request.
      const n = keepalive ? scope.requests : ++scope.requests;
      const send = Effect.tryPromise({
        try: () => options.send(id, body, keepalive),
        catch: (cause) => cause,
      });
      return Effect.matchEffect(send, {
        onSuccess: (result) =>
          Effect.sync(() => {
            const confirmed = options.confirm?.(id, changes, result) ?? changes;
            confirmed.forEach((c) => {
              slotOf(scope, c.key).confirmed = { value: c.value };
            });
            if (n === scope.requests) options.answered?.(id, result);
          }),
        onFailure: () =>
          Effect.sync(() => {
            changes.forEach((c) => {
              const slot = slotOf(scope, c.key);
              if (slot.wanted === undefined || slot.wanted.value !== c.value) return;
              delete slot.wanted;
              if (slot.confirmed !== undefined) notify(scope, c.key, slot.confirmed.value);
            });
            options.failed?.(id, changes);
          }),
      }).pipe(Effect.ensuring(Effect.sync(() => {
        changes.forEach((c) => {
          const slot = slotOf(scope, c.key);
          const at = slot.carrying.indexOf(c.value);
          if (at >= 0) slot.carrying.splice(at, 1);
          if (
            slot.wanted !== undefined && !slot.carrying.length && slot.confirmed !== undefined
            && slot.confirmed.value === slot.wanted.value
          ) delete slot.wanted;
        });
        if (anyPending(scope)) schedule(id, scope);
      })));
    });
  }

  return {
    set(id, key, value) {
      const scope = scopeOf(id);
      const slot = slotOf(scope, key);
      // A wish for what the server already has, with nothing in flight, is
      // no wish at all; while something is in flight it is kept, since what
      // the server has is about to change.
      if (!slot.carrying.length && slot.confirmed?.value === value) delete slot.wanted;
      else slot.wanted = { value };
      // The save is queued before subscribers draw, so one that throws
      // cannot leave the wish unsaved; the send still waits for this task.
      if (pending(slot)) schedule(id, scope);
      notify(scope, key, value);
    },

    seed(id, key, confirmed) {
      const scope = scopeOf(id);
      const slot = slotOf(scope, key);
      if (slot.wanted !== undefined || slot.carrying.length) return false;
      const before = slot.confirmed;
      slot.confirmed = { value: confirmed };
      if (before === undefined || before.value !== confirmed) notify(scope, key, confirmed);
      return true;
    },

    get(id, key) {
      return current(scopes.get(id)?.keys.get(key))?.value;
    },

    subscribe(id, key, render, signal) {
      if (signal.aborted) return;
      const scope = scopeOf(id);
      const sub: Sub<V> = { key, render };
      scope.subs.add(sub);
      scope.subscribed = true;
      signal.addEventListener("abort", () => {
        scope.subs.delete(sub);
        evict(scope);
      }, { once: true });
      if (key !== null) {
        const now = current(scope.keys.get(key));
        if (now !== undefined) render(key, now.value);
        return;
      }
      scope.keys.forEach((slot, k) => {
        const now = current(slot);
        if (now !== undefined) render(k, now.value);
      });
    },

    settled(id) {
      return Effect.runPromise(Effect.gen(function*() {
        for (;;) {
          const scope = scopes.get(id);
          if (!scope || scope.active.size === 0) return;
          yield* Fiber.awaitAll([...scope.active]);
        }
      }));
    },

    run(id, fn) {
      const fiber = enqueue(scopeOf(id), Effect.tryPromise({ try: fn, catch: (cause) => cause }));
      return Effect.runPromise(Fiber.join(fiber));
    },

    flush() {
      scopes.forEach((scope, id) => {
        const changes = sendable(scope);
        if (!changes.length) return;
        const body = options.fold(changes);
        if (body === null) return;
        start(scope, dispatch(id, scope, changes, body, true));
      });
    },
  };
}

type LeaveTarget = {
  addEventListener: (type: "pagehide", listener: () => void) => void;
  document: {
    addEventListener: (type: "visibilitychange", listener: () => void) => void;
    visibilityState: string;
  };
};

// installLeaveFlush flushes the store when the page is being left and when
// it is hidden, which on a phone is often the last event before the browser
// is discarded. Kept apart from the store so the store has no DOM; it also
// makes the in-flight ordering limit (a change to a key already in flight,
// then a tab switch) reachable without leaving the page.
export function installLeaveFlush(store: { flush: () => void }, target: LeaveTarget = window): void {
  target.addEventListener("pagehide", () => store.flush());
  target.document.addEventListener("visibilitychange", () => {
    if (target.document.visibilityState === "hidden") store.flush();
  });
}

declare global {
  interface Window {
    HousecatIntents: {
      create: typeof createIntentStore;
      installLeaveFlush: typeof installLeaveFlush;
      optimistic: typeof createOptimisticState;
    };
  }
}

if (typeof window !== "undefined") {
  window.HousecatIntents = {
    create: createIntentStore,
    installLeaveFlush,
    optimistic: createOptimisticState,
  };
}
