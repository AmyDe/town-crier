import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import {
  fetchJSONWithRetry,
  mapWithConcurrency,
  gatherSnapshot,
  runFetch,
  resolveFetchTuning,
  DEFAULT_FETCH_TIMEOUT_MS,
  DEFAULT_FETCH_RETRIES,
  DEFAULT_FETCH_CONCURRENCY,
} from '../prerender-planning.mjs';

const silentLogger = { log() {}, warn() {}, error() {} };
const noBackoff = () => Promise.resolve();

/** A minimal `Response`-like object for a hand-written fetch fake. */
function jsonResponse(status, body) {
  return { ok: status >= 200 && status < 300, status, json: async () => body };
}

async function exists(path) {
  try {
    await access(path);
    return true;
  } catch {
    return false;
  }
}

// ── fetchJSONWithRetry ──────────────────────────────────────────────────────
describe('fetchJSONWithRetry', () => {
  it('returns the parsed JSON body on a first-attempt 200', async () => {
    let calls = 0;
    const fetchImpl = async () => {
      calls += 1;
      return jsonResponse(200, { hello: 'world' });
    };

    const body = await fetchJSONWithRetry('https://api.example/x', {
      fetchImpl,
      headers: { 'X-Build-Key': 'k' },
      sleepImpl: noBackoff,
    });

    expect(body).toEqual({ hello: 'world' });
    expect(calls).toBe(1);
  });

  it('passes headers and an abort signal through to fetchImpl', async () => {
    const seen = [];
    const fetchImpl = async (url, init) => {
      seen.push({ url, init });
      return jsonResponse(200, {});
    };

    await fetchJSONWithRetry('https://api.example/x', {
      fetchImpl,
      headers: { 'X-Build-Key': 'secret' },
      sleepImpl: noBackoff,
    });

    expect(seen).toHaveLength(1);
    expect(seen[0].url).toBe('https://api.example/x');
    expect(seen[0].init.headers).toEqual({ 'X-Build-Key': 'secret' });
    expect(seen[0].init.signal).toBeInstanceOf(AbortSignal);
  });

  it.each([
    ['a network rejection', () => Promise.reject(new Error('fetch failed'))],
    ['an HTTP 500', () => Promise.resolve(jsonResponse(500, {}))],
    ['an HTTP 429', () => Promise.resolve(jsonResponse(429, {}))],
  ])('retries after %s and succeeds on the second attempt', async (_label, firstAttempt) => {
    let calls = 0;
    const fetchImpl = () => {
      calls += 1;
      return calls === 1 ? firstAttempt() : Promise.resolve(jsonResponse(200, { ok: true }));
    };

    const body = await fetchJSONWithRetry('https://api.example/x', {
      fetchImpl,
      sleepImpl: noBackoff,
    });

    expect(body).toEqual({ ok: true });
    expect(calls).toBe(2);
  });

  it('throws after exhausting all attempts on repeated 500s, naming the URL', async () => {
    let calls = 0;
    const fetchImpl = async () => {
      calls += 1;
      return jsonResponse(500, {});
    };

    await expect(
      fetchJSONWithRetry('https://api.example/authorities/9', {
        fetchImpl,
        retries: DEFAULT_FETCH_RETRIES,
        sleepImpl: noBackoff,
      }),
    ).rejects.toThrow(
      /https:\/\/api\.example\/authorities\/9 failed after 3 attempt\(s\): HTTP 500/,
    );
    expect(calls).toBe(3);
  });

  it('throws immediately on a non-429 4xx, with exactly one fetch call', async () => {
    let calls = 0;
    const fetchImpl = async () => {
      calls += 1;
      return jsonResponse(403, {});
    };

    await expect(
      fetchJSONWithRetry('https://api.example/x', { fetchImpl, sleepImpl: noBackoff }),
    ).rejects.toThrow(/HTTP 403/);
    expect(calls).toBe(1);
  });

  it('aborts and rejects a request that never resolves once the timeout elapses', async () => {
    vi.useFakeTimers();
    try {
      let aborted = false;
      const fetchImpl = (_url, init) =>
        new Promise((_resolve, reject) => {
          init.signal.addEventListener('abort', () => {
            aborted = true;
            reject(new Error('aborted'));
          });
        });

      const settled = fetchJSONWithRetry('https://api.example/slow', {
        fetchImpl,
        timeoutMs: DEFAULT_FETCH_TIMEOUT_MS,
        retries: 1,
        sleepImpl: noBackoff,
      }).catch((err) => err);

      await vi.advanceTimersByTimeAsync(DEFAULT_FETCH_TIMEOUT_MS);

      const err = await settled;
      expect(err).toBeInstanceOf(Error);
      expect(err.message).toMatch(/failed after 1 attempt\(s\): request timed out after 30000ms/);
      expect(aborted).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });
});

// ── mapWithConcurrency ──────────────────────────────────────────────────────
describe('mapWithConcurrency', () => {
  it('never runs more than the pool size at once', async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    const worker = async (n) => {
      inFlight += 1;
      maxInFlight = Math.max(maxInFlight, inFlight);
      await new Promise((resolve) => setTimeout(resolve, 5));
      inFlight -= 1;
      return n;
    };

    const items = Array.from({ length: 20 }, (_, i) => i);
    const results = await mapWithConcurrency(items, 4, worker);

    expect(maxInFlight).toBe(4);
    expect(results).toEqual(items);
  });

  it('returns results in input order even when later items settle first', async () => {
    const items = [10, 11, 12, 13, 14, 15];
    const worker = (n, i) =>
      new Promise((resolve) => {
        // earlier index -> longer delay, so earlier items finish LAST
        setTimeout(() => resolve(`r${n}`), (items.length - i) * 8);
      });

    const results = await mapWithConcurrency(items, items.length, worker);

    expect(results).toEqual(['r10', 'r11', 'r12', 'r13', 'r14', 'r15']);
  });

  it('rejects on the first worker rejection and stops starting new items', async () => {
    const started = [];
    const worker = async (n) => {
      started.push(n);
      if (n === 2) {
        throw new Error('boom on 2');
      }
      return n;
    };

    await expect(
      mapWithConcurrency([1, 2, 3, 4, 5, 6], 2, worker),
    ).rejects.toThrow('boom on 2');

    expect(started).not.toContain(6);
    expect(started.length).toBeLessThan(6);
  });

  it('is a no-op for an empty item list', async () => {
    let calls = 0;
    const results = await mapWithConcurrency([], 4, async () => {
      calls += 1;
    });
    expect(results).toEqual([]);
    expect(calls).toBe(0);
  });
});

// ── resolveFetchTuning ──────────────────────────────────────────────────────
describe('resolveFetchTuning', () => {
  it('uses the conservative defaults when the env is empty', () => {
    expect(resolveFetchTuning({})).toEqual({
      timeoutMs: DEFAULT_FETCH_TIMEOUT_MS,
      retries: DEFAULT_FETCH_RETRIES,
      concurrency: DEFAULT_FETCH_CONCURRENCY,
    });
  });

  it('reads valid overrides', () => {
    expect(
      resolveFetchTuning({
        PRERENDER_FETCH_TIMEOUT_MS: '5000',
        PRERENDER_FETCH_RETRIES: '2',
        PRERENDER_FETCH_CONCURRENCY: '8',
      }),
    ).toEqual({ timeoutMs: 5000, retries: 2, concurrency: 8 });
  });

  it('clamps concurrency to the 1..16 band and rejects junk', () => {
    expect(resolveFetchTuning({ PRERENDER_FETCH_CONCURRENCY: '100' }).concurrency).toBe(16);
    expect(resolveFetchTuning({ PRERENDER_FETCH_CONCURRENCY: '0' }).concurrency).toBe(
      DEFAULT_FETCH_CONCURRENCY,
    );
    expect(resolveFetchTuning({ PRERENDER_FETCH_CONCURRENCY: 'lots' }).concurrency).toBe(
      DEFAULT_FETCH_CONCURRENCY,
    );
  });
});

// ── gatherSnapshot: bounded concurrency + loud failure ──────────────────────
describe('gatherSnapshot', () => {
  const authoritiesOf = (n) =>
    Array.from({ length: n }, (_, i) => ({
      id: i + 1,
      name: `Authority ${i + 1}`,
      areaType: 'English District',
    }));
  const townsOf = (n) =>
    Array.from({ length: n }, (_, i) => ({
      slug: `town-${i + 1}`,
      name: `Town ${i + 1}`,
      lat: 50 + i / 1000,
      lng: -1 - i / 1000,
      authorityId: 1,
      population: 50000,
    }));

  it('fetches with bounded concurrency (default 4) and preserves input order', async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    const fetchImpl = async () => {
      inFlight += 1;
      maxInFlight = Math.max(maxInFlight, inFlight);
      await new Promise((resolve) => setTimeout(resolve, 4));
      inFlight -= 1;
      return jsonResponse(200, {
        areaName: 'X',
        applications: [],
        total: 11,
        statusBreakdown: [],
      });
    };

    const snapshot = await gatherSnapshot({
      apiBase: 'https://api.example',
      buildKey: 'k',
      limit: 30,
      minPopulation: 20000,
      fetchImpl,
      loadAuthorities: async () => authoritiesOf(20),
      loadTowns: async () => townsOf(20),
      now: () => '2026-09-03T00:00:00.000Z',
      sleepImpl: noBackoff,
    });

    expect(maxInFlight).toBe(DEFAULT_FETCH_CONCURRENCY);
    expect(snapshot.authorityPages.map((a) => a.id)).toEqual(
      authoritiesOf(20).map((a) => a.id),
    );
    expect(snapshot.townPages.map((t) => t.slug)).toEqual(
      townsOf(20).map((t) => t.slug),
    );
  });

  it('honours a lower concurrency override', async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    const fetchImpl = async () => {
      inFlight += 1;
      maxInFlight = Math.max(maxInFlight, inFlight);
      await new Promise((resolve) => setTimeout(resolve, 4));
      inFlight -= 1;
      return jsonResponse(200, { applications: [], total: 5, statusBreakdown: [] });
    };

    await gatherSnapshot({
      apiBase: 'https://api.example',
      buildKey: 'k',
      limit: 30,
      minPopulation: 20000,
      fetchImpl,
      loadAuthorities: async () => [],
      loadTowns: async () => townsOf(12),
      now: () => '2026-09-03T00:00:00.000Z',
      concurrency: 2,
      sleepImpl: noBackoff,
    });

    expect(maxInFlight).toBe(2);
  });

  it('rejects the whole gather when one request keeps failing after retries', async () => {
    const fetchImpl = async (url) => {
      if (url.includes('/v1/authorities/2/')) {
        return jsonResponse(500, {});
      }
      return jsonResponse(200, {
        areaName: 'ok',
        applications: [],
        total: 12,
        statusBreakdown: [],
      });
    };

    await expect(
      gatherSnapshot({
        apiBase: 'https://api.example',
        buildKey: 'k',
        limit: 30,
        minPopulation: 20000,
        fetchImpl,
        loadAuthorities: async () => authoritiesOf(6),
        loadTowns: async () => [],
        now: () => '2026-09-03T00:00:00.000Z',
        retries: 2,
        sleepImpl: noBackoff,
      }),
    ).rejects.toThrow(/\/v1\/authorities\/2\/applications.*HTTP 500/s);
  });
});

// ── runFetch: never writes a partial snapshot ───────────────────────────────
describe('runFetch — partial snapshot guard', () => {
  let outDir;

  beforeEach(async () => {
    outDir = await mkdtemp(join(tmpdir(), 'prerender-planning-'));
  });

  afterEach(async () => {
    await rm(outDir, { recursive: true, force: true });
  });

  it('rejects and writes no snapshot file when one request fails permanently', async () => {
    const snapshotPath = join(outDir, 'seo-snapshot.json');
    const fetchImpl = async (url) => {
      if (url.includes('/v1/authorities/3/')) {
        return jsonResponse(503, {});
      }
      return jsonResponse(200, {
        areaName: 'ok',
        applications: [],
        total: 15,
        statusBreakdown: [],
      });
    };

    await expect(
      runFetch({
        snapshotPath,
        apiBase: 'https://api.example',
        buildKey: 'k',
        env: { PRERENDER_FETCH_RETRIES: '2' },
        fetchImpl,
        loadAuthorities: async () => [
          { id: 1, name: 'One', areaType: 'English District' },
          { id: 2, name: 'Two', areaType: 'English District' },
          { id: 3, name: 'Three', areaType: 'English District' },
          { id: 4, name: 'Four', areaType: 'English District' },
        ],
        loadTowns: async () => [],
        now: () => '2026-09-03T00:00:00.000Z',
        sleepImpl: noBackoff,
        logger: silentLogger,
      }),
    ).rejects.toThrow(/HTTP 503/);

    expect(await exists(snapshotPath)).toBe(false);
  });
});
