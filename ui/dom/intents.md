# Intent store

`createIntentStore` tracks wanted and confirmed values per scope and key. Its
synchronous API updates subscribers immediately. The adapter supplies request
folding, sending, and response handling; reader and list integrations can share
one store without teaching it about the DOM or Gmail.

Effect owns asynchronous work:

- A semaphore with one permit serializes each scope independently.
- A scheduler yield lets changes made in the same JavaScript task coalesce before
  the queued request body is folded.
- Fibers represent queued and running work. `settled` awaits their exits, including
  failures, and checks again for work created during completion.
- `Effect.tryPromise` adapts the sender and public `run` callback. `runPromise`
  implements the existing Promise-returning public API.
- `Effect.matchEffect` confirms or rolls back a request, while `Effect.ensuring`
  releases its carried values and schedules remaining changes even if a callback
  throws. The semaphore also releases its permit on every outcome.

Unsubscribing does not cancel saves. A scope is evicted only after its last view
leaves and it has no queued, running, or outstanding work. A page-leave flush
starts the sender synchronously with `keepalive`, bypassing the semaphore so it
does not wait behind another request. It does not suppress that request's answer
or resend values already carried by an in-flight request. Failed sends roll back;
the store does not introduce automatic retries.

## Optimistic reducers

`createOptimisticState` (also `HousecatIntents.optimistic`) keeps a versioned
authoritative value separate from pending pure reducers. `set(id, reducer)` adds
or replaces an operation; `get()` folds pending reducers over the current base.
`receive(snapshot, includedIDs)` retires operations represented in that snapshot,
accepts the snapshot only when its version is not older, and leaves the remaining
reducers to run against the new value. `remove(id)` rolls back a failed operation.
Reducers must return new values rather than mutate their input.

Included IDs are server acknowledgements, not a guess from response order. A
newer snapshot can include writes whose individual responses have not arrived.
For an increment example, pending A/B/C shows 3; a response containing an external
increment and A gives a base of 2 plus B/C, showing 4 throughout later responses.

The mail adapter uses these reducers for unread counts. Its requests carry a
page-client UUID and increasing sequence per account. The server persists receipts
and per-conversation, per-label sequence fences. A stale assignment is acknowledged
without overwriting a newer assignment; unrelated labels still apply. Duplicate
requests return current counts without repeating their writes. The adapter's
`confirm` hook maps superseded writes to their acknowledged values, so they do not
trigger unnecessary corrective sends. Ordering is per client; competing clients
retain server application order. Generic command adapters must supply their own
ordering/idempotency policy; client reducers do not order server writes.

Tracked mutations are serialized per client/account through the provider write
and count snapshot, so each snapshot's receipts describe the same completed set.
Different clients remain independent. This trades parallel writes from one page
for consistent acknowledgement metadata.

A failed fetch does not advance the client's acknowledged sequence. A browser
can reject the response during navigation while the keepalive request still
reaches the server; retiring it would make a later flush suppress that write.

## Sidebar labels

`labelintents.ts` is the label menu's adapter, one scope per account, keyed
`field:labelID` for `list`, `message`, `edit` (the own name and parent, as the
Edit label dialog posts them) and `remove`. Its projection lays wanted values
over the labels as the server drew them (each ⋮'s data, which script never
writes), renaming a label's subtree with it and taking a removed label's
subtree out, so `labelmenu.ts` can draw the sidebar and mail-row chips at once
and again over any sidebar or rows the server swaps in. Pending changes fold
into one `/mail/labels/visibility` batch, then each edit, then each removal,
in one turn of the scope's lane. Each is posted through htmx (`htmxWrite`,
`htmx_write.ts`), and every endpoint answers with the sidebar out of band,
which htmx swaps in. A run Gmail stopped partway also brings a toast saying
what was not done and raises `mail:label-writes-stopped`, which ends the
turn. Once the account settles, the store is seeded again from the sidebar,
so a step Gmail refused partway shows as Gmail has it. A refusal before
anything changed rolls back and raises the request-failure toast. A
page-leave flush starts only the first of those requests synchronously.

## Runtime and tests

Effect installs from the lockfile, as a dependency of `ui/`. The store imports
`effect/Effect`, `effect/Fiber`, and `effect/Semaphore`, which Vite+ bundles
into `ui.js` from Effect's published build, keeping only the functions the store
calls. The factory
takes those functions as an optional `IntentRuntime`; its default names each one,
because a module passed whole would keep every export, and tests may pass their own.

`make -C app lint-ts` type-checks the store against the installed package's
declarations, where `ui/tsconfig.json` maps `effect/*`, with Housecat's own stricter
return-path checks.

Unit tests import the same installed Effect the bundle does. Run:

```sh
make -C app lint-ts
make -C app test-ts
```

The tests cover coalescing, ordered sends, independent scopes, rollback, callback
failures, eviction, subscriptions, and page-leave flushing.
