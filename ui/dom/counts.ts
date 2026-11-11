import { createOptimisticState } from "./optimistic";

type Version = readonly number[];
type Snapshot = { text: string; version: Version };
type Contribution = { current: number; unread: number };
type Contributions = Record<string, Contribution>;
type Value = { count: number; contributions: Record<string, Contribution> };
type Receipt = {
  additional: readonly number[] | null;
  contributions: Record<string, Contributions | null> | null;
  through: number;
};
type Result = {
  contributions?: Contributions | null;
  counts?: Record<string, Snapshot> | null;
  intent?: Receipt;
};
type Wish = { generation: number; unread: boolean };
type Request = { generation: number; thread: string; unread: boolean | undefined };
type Counter = {
  snapshot: Snapshot;
  state: ReturnType<typeof createOptimisticState<Value, Version>>;
};
type Account = {
  confirmed: Record<string, Contributions>;
  counters: Map<string, Counter>;
  generation: number;
  requests: Map<number, Request>;
  wishes: Map<string, Wish>;
};

function compare(a: Version, b: Version): number {
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    const difference = (a[i] ?? 0) - (b[i] ?? 0);
    if (difference) return difference;
  }
  return 0;
}

function number(text: string): number | undefined {
  return /^[\d,]*$/.test(text) ? Number(text.replaceAll(",", "")) : undefined;
}

export function createCountState() {
  const accounts = new Map<string, Account>();

  function accountOf(id: string): Account {
    let account = accounts.get(id);
    if (!account) {
      account = { confirmed: {}, counters: new Map(), generation: 0, requests: new Map(), wishes: new Map() };
      accounts.set(id, account);
    }
    return account;
  }

  function project(account: Account): void {
    for (const counter of account.counters.values()) {
      for (const id of counter.state.pending()) counter.state.remove(id);
      for (const [thread, wish] of account.wishes) {
        counter.state.set(thread, (value) => {
          const contribution = value.contributions[thread] as Contribution | undefined;
          if (!contribution) return value;
          const current = wish.unread ? contribution.unread : 0;
          return {
            count: value.count + current - contribution.current,
            contributions: { ...value.contributions, [thread]: { ...contribution, current } },
          };
        });
      }
    }
  }

  function merge(
    account: Account,
    snapshots: Record<string, Snapshot>,
    confirmed?: Record<string, Contributions | null>,
  ): void {
    for (const [key, snapshot] of Object.entries(snapshots)) {
      const previous = account.counters.get(key);
      if (previous && compare(snapshot.version, previous.snapshot.version) < 0) continue;
      for (const [thread, values] of Object.entries(confirmed ?? {})) {
        const known = account.confirmed[thread] ?? (account.confirmed[thread] = {});
        if (values?.[key]) known[key] = values[key];
        else delete known[key];
      }
      const contributions: Record<string, Contribution> = {};
      for (const [thread, values] of Object.entries(account.confirmed)) {
        const contribution = values[key] as Contribution | undefined;
        if (contribution) contributions[thread] = contribution;
      }
      const value = { count: number(snapshot.text) ?? 0, contributions };
      if (previous) {
        if (previous.state.receive({ value, version: snapshot.version }, [])) previous.snapshot = snapshot;
      } else {
        account.counters.set(key, {
          snapshot,
          state: createOptimisticState({ value, version: snapshot.version }, compare),
        });
      }
    }
    project(account);
  }

  return {
    merge(id: string, snapshots: Record<string, Snapshot>): void {
      const account = accountOf(id);
      // An unacknowledged snapshot cannot say which pending writes it includes.
      if (account.requests.size || account.wishes.size) return;
      merge(
        account,
        Object.fromEntries(
          Object.entries(snapshots).filter(([key, snapshot]) => {
            const previous = account.counters.get(key);
            return !previous || compare(snapshot.version, previous.snapshot.version) > 0;
          }),
        ),
      );
    },
    seed(id: string, thread: string, contributions: Contributions, snapshots?: Record<string, Snapshot>): void {
      const account = accountOf(id);
      if (account.wishes.has(thread)) return;
      const confirmed = account.confirmed[thread] as Contributions | undefined;
      if (!snapshots && confirmed) return;
      const known = confirmed ?? (account.confirmed[thread] = {});
      for (const [key, counter] of account.counters) {
        const snapshot = snapshots?.[key];
        if (snapshots && (!snapshot || compare(snapshot.version, counter.snapshot.version) !== 0)) continue;
        const contribution = contributions[key] as Contribution | undefined;
        if (contribution) known[key] = contribution;
        else delete known[key];
      }
      merge(account, Object.fromEntries([...account.counters].map(([key, counter]) => [key, counter.snapshot])));
    },
    wish(id: string, thread: string, unread: boolean): void {
      const account = accountOf(id);
      account.wishes.set(thread, { generation: ++account.generation, unread });
      if (
        ![...account.requests.values()].some((request) => request.thread === thread)
        && Object.values(account.confirmed[thread] ?? {}).every((value) =>
          value.current === (unread ? value.unread : 0)
        )
      ) account.wishes.delete(thread);
      project(account);
    },
    sent(id: string, thread: string, sequence: number, unread: boolean | undefined): void {
      const account = accountOf(id);
      account.requests.set(sequence, { generation: account.wishes.get(thread)?.generation ?? 0, thread, unread });
    },
    receive(id: string, sequence: number, result: Result | undefined): void {
      const account = accountOf(id);
      const request = account.requests.get(sequence);
      const receipt = result?.intent;
      const included = receipt
        ? [...account.requests.keys()].filter((n) => n <= receipt.through || receipt.additional?.includes(n) === true)
        : [sequence];
      for (const n of included) {
        const completed = account.requests.get(n);
        if (!completed) continue;
        if (
          completed.unread !== undefined && account.wishes.get(completed.thread)?.generation === completed.generation
        ) {
          account.wishes.delete(completed.thread);
        }
        account.requests.delete(n);
      }
      if (result?.counts) {
        merge(
          account,
          result.counts,
          receipt?.contributions ?? (request ? { [request.thread]: result.contributions ?? null } : {}),
        );
      } else if (request?.unread !== undefined) {
        const contributions = account.confirmed[request.thread] ?? {};
        for (const [key, counter] of account.counters) {
          const contribution = contributions[key] as Contribution | undefined;
          if (!contribution) continue;
          const current = request.unread ? contribution.unread : 0;
          const count = counter.state.base().count + current - contribution.current;
          contributions[key] = { ...contribution, current };
          counter.snapshot = { ...counter.snapshot, text: count ? count.toLocaleString("en-US") : "" };
        }
        account.confirmed[request.thread] = contributions;
        merge(account, Object.fromEntries([...account.counters].map(([key, counter]) => [key, counter.snapshot])));
      } else project(account);
    },
    failed(id: string, sequence: number): void {
      const account = accountOf(id);
      const request = account.requests.get(sequence);
      if (!request) return;
      account.requests.delete(sequence);
      if (account.wishes.get(request.thread)?.generation === request.generation) account.wishes.delete(request.thread);
      project(account);
    },
    rollback(id: string, thread: string, unread: boolean): void {
      const account = accountOf(id);
      if (![...account.requests.values()].some((request) => request.thread === thread)) account.wishes.delete(thread);
      else account.wishes.set(thread, { generation: ++account.generation, unread });
      project(account);
    },
    values(id: string): Record<string, string> {
      const account = accountOf(id);
      return Object.fromEntries([...account.counters].map(([key, counter]) => {
        const value = Math.max(0, counter.state.get().count);
        const text = number(counter.snapshot.text) === undefined || value === counter.state.base().count
          ? counter.snapshot.text
          : value
          ? value.toLocaleString("en-US")
          : "";
        return [key, text];
      }));
    },
  };
}

declare global {
  interface Window {
    HousecatCounts: { create: typeof createCountState };
  }
}

if (typeof window !== "undefined") window.HousecatCounts = { create: createCountState };
