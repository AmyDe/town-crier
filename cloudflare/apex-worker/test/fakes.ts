import type { Deps, EdgeCache, Env } from "../src/route";

export const env: Env = {
  SWA_ORIGIN: "https://swa-default.azurestaticapps.net",
  API_ORIGIN: "https://api.example.test",
  INDEXABLE_HOSTS: "towncrierapp.uk",
  SITE_BUILD_KEY: "build-key-secret",
};

export interface FetchCall {
  url: string;
  init: RequestInit | undefined;
}

type Responder = (url: string, init: RequestInit | undefined) => Promise<Response>;

export class FakeFetch {
  calls: FetchCall[] = [];
  constructor(private responder: Responder = async () => new Response("ok")) {}

  respondWith(responder: Responder): void {
    this.responder = responder;
  }

  fetch = (url: string, init?: RequestInit): Promise<Response> => {
    this.calls.push({ url, init });
    return this.responder(url, init);
  };
}

export class FakeCache implements EdgeCache {
  store = new Map<string, Response>();
  puts: string[] = [];

  async match(key: string): Promise<Response | undefined> {
    return this.store.get(key)?.clone();
  }

  async put(key: string, response: Response): Promise<void> {
    this.puts.push(key);
    this.store.set(key, response.clone());
  }
}

export class FakeContext {
  pending: Promise<unknown>[] = [];
  waitUntil = (promise: Promise<unknown>): void => {
    this.pending.push(promise);
  };
  async settle(): Promise<void> {
    await Promise.all(this.pending);
  }
}

export class FakeClock {
  constructor(public nowMs: number) {}
  now = (): number => this.nowMs;
  advanceSeconds(seconds: number): void {
    this.nowMs += seconds * 1000;
  }
}

export function makeDeps(overrides: Partial<{ fetch: FakeFetch; cache: FakeCache; clock: FakeClock; ctx: FakeContext; originTimeoutMs: number }> = {}) {
  const fetch = overrides.fetch ?? new FakeFetch();
  const cache = overrides.cache ?? new FakeCache();
  const clock = overrides.clock ?? new FakeClock(Date.UTC(2026, 9, 8, 12, 0, 0));
  const ctx = overrides.ctx ?? new FakeContext();
  const deps: Deps = {
    fetch: fetch.fetch,
    cache,
    now: clock.now,
    waitUntil: ctx.waitUntil,
    originTimeoutMs: overrides.originTimeoutMs,
  };
  return { fetch, cache, clock, ctx, deps };
}

export function headerOf(init: RequestInit | undefined, name: string): string | null {
  return new Headers(init?.headers).get(name);
}
