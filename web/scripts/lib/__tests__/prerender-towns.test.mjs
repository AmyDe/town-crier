import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, readFile, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  runPrerender,
  loadTownsFromFile,
  resolveMinPopulation,
} from '../../prerender-planning.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const AUTHORITY_FIXTURE = join(
  here,
  '..',
  '..',
  'fixtures',
  'sample-authorities.json',
);
const TOWN_FIXTURE = join(here, '..', '..', 'fixtures', 'sample-towns.json');

/** Hand-written fetch stub — no vi.fn / vi.mock. */
class StubFetch {
  /** @param {(url: string, init?: object) => { ok: boolean, status: number, body: unknown }} handler */
  constructor(handler) {
    this.calls = [];
    this.handler = handler;
  }

  fetch = async (url, init) => {
    this.calls.push({ url: String(url), init });
    const { ok, status, body } = this.handler(String(url), init);
    return { ok, status, json: async () => body };
  };
}

const silentLogger = { log() {}, warn() {}, error() {} };

let outDir;

beforeEach(async () => {
  outDir = await mkdtemp(join(tmpdir(), 'prerender-towns-'));
});

afterEach(async () => {
  await rm(outDir, { recursive: true, force: true });
});

async function exists(path) {
  try {
    await access(path);
    return true;
  } catch {
    return false;
  }
}

describe('runPrerender — town fixture mode', () => {
  it('emits a nested /planning/<authority>/<town>/index.html for a gated town', async () => {
    const result = await runPrerender({
      outDir,
      townFixturePath: TOWN_FIXTURE,
      logger: silentLogger,
    });

    expect(result.publishedTowns).toEqual(['cornwall/truro']);

    const html = await readFile(
      join(outDir, 'planning', 'cornwall', 'truro', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<h1>Planning applications in Truro</h1>');
    expect(html).toContain('PlanIt');
    expect(html).toContain('Get push alerts for Truro');
    expect(html).toContain(
      '<link rel="canonical" href="https://towncrierapp.uk/planning/cornwall/truro"',
    );
  });

  it('excludes a town below the coverage gate', async () => {
    const result = await runPrerender({
      outDir,
      townFixturePath: TOWN_FIXTURE,
      logger: silentLogger,
    });

    expect(
      await exists(join(outDir, 'planning', 'cornwall', 'sparse-village')),
    ).toBe(false);
    expect(result.excludedTowns.map((e) => e.name)).toContain('Sparse Village');
  });

  it('the town page does not collide with the bare authority page path', async () => {
    await runPrerender({
      outDir,
      townFixturePath: TOWN_FIXTURE,
      logger: silentLogger,
    });
    // Town page is nested two levels deep; an authority page would be a sibling
    // index.html directly under planning/cornwall (not emitted in town-only mode).
    expect(
      await exists(join(outDir, 'planning', 'cornwall', 'truro', 'index.html')),
    ).toBe(true);
    expect(await exists(join(outDir, 'planning', 'cornwall', 'index.html'))).toBe(
      false,
    );
  });
});

describe('runPrerender — sitemap with authority and town pages', () => {
  it('lists both authority pages and nested town pages', async () => {
    await runPrerender({
      outDir,
      fixturePath: AUTHORITY_FIXTURE,
      townFixturePath: TOWN_FIXTURE,
      logger: silentLogger,
    });

    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/basingstoke-and-deane</loc>',
    );
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/cornwall/truro</loc>',
    );
  });
});

describe('runPrerender — town live mode', () => {
  const cornwallTowns = [
    { slug: 'truro', name: 'Truro', lat: 50.2632, lng: -5.051, authorityId: 52 },
  ];
  // areaType is deliberately non-qualifying here so the authority pass is a
  // no-op and these tests isolate the TOWN pipeline. Slug resolution
  // (authorityId 52 -> "Cornwall" -> "cornwall") ignores areaType.
  const cornwallAuthorities = [
    { id: 52, name: 'Cornwall', areaType: 'English County' },
  ];

  it('calls the geo endpoint with authorityId+lat+lng and X-Build-Key, then emits the nested page', async () => {
    const stub = new StubFetch((url) => {
      if (url.includes('/v1/applications/near')) {
        return {
          ok: true,
          status: 200,
          body: {
            authorityId: 52,
            lat: 50.2632,
            lng: -5.051,
            radius: 5000,
            applications: [
              {
                uid: 'CW1',
                name: 'PA26/0001',
                address: 'Lemon Quay, Truro',
                description: 'Café conversion',
                appState: 'Permitted',
                startDate: '2026-01-12',
                lastDifferent: '2026-06-12T10:00:00+00:00',
                link: 'https://planit.org.uk/planapplic/CW1',
                url: null,
              },
            ],
            total: 14,
            statusBreakdown: [{ appState: 'Permitted', count: 14 }],
          },
        };
      }
      throw new Error(`unexpected url ${url}`);
    });

    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => cornwallTowns,
      logger: silentLogger,
    });

    expect(result.publishedTowns).toEqual(['cornwall/truro']);

    const nearCalls = stub.calls.filter((c) => c.url.includes('/v1/applications/near'));
    expect(nearCalls).toHaveLength(1);
    expect(nearCalls[0].url).toContain('authorityId=52');
    expect(nearCalls[0].url).toContain('lat=50.2632');
    expect(nearCalls[0].url).toContain('lng=-5.051');
    expect(nearCalls[0].url).toContain('limit=30');
    // Selects the nearest-N by distance (the order=distance variant from tc-2avw.1).
    expect(nearCalls[0].url).toContain('order=distance');
    expect(nearCalls[0].init.headers['X-Build-Key']).toBe('test-key');

    const html = await readFile(
      join(outDir, 'planning', 'cornwall', 'truro', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<h1>Planning applications in Truro</h1>');
  });

  it('excludes a town that fails the coverage gate', async () => {
    const stub = new StubFetch(() => ({
      ok: true,
      status: 200,
      body: {
        authorityId: 52,
        lat: 50.2632,
        lng: -5.051,
        radius: 5000,
        applications: [],
        total: 3,
        statusBreakdown: [],
      },
    }));

    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => cornwallTowns,
      logger: silentLogger,
    });

    expect(result.publishedTowns).toEqual([]);
    expect(await exists(join(outDir, 'planning', 'cornwall', 'truro'))).toBe(false);
  });

  it('treats zero towns as a valid (non-error) build with no geo calls', async () => {
    const stub = new StubFetch(() => {
      throw new Error('geo endpoint must not be called');
    });

    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => [],
      logger: silentLogger,
    });

    expect(result.skipped).toBe(false);
    expect(result.publishedTowns).toEqual([]);
    expect(stub.calls).toHaveLength(0);
  });

  it('fails loud when the geo endpoint returns an unexpected shape', async () => {
    const stub = new StubFetch(() => ({
      ok: true,
      status: 200,
      body: { not: 'what we expect' },
    }));

    await expect(
      runPrerender({
        outDir,
        apiBase: 'https://api-dev.towncrierapp.uk',
        buildKey: 'test-key',
        fetchImpl: stub.fetch,
        loadAuthorities: async () => cornwallAuthorities,
        loadTowns: async () => cornwallTowns,
        logger: silentLogger,
      }),
    ).rejects.toThrow();
  });

  it('fails loud when the geo endpoint omits statusBreakdown', async () => {
    const stub = new StubFetch(() => ({
      ok: true,
      status: 200,
      body: {
        authorityId: 52,
        lat: 50.2632,
        lng: -5.051,
        radius: 5000,
        applications: [],
        total: 14,
      },
    }));

    await expect(
      runPrerender({
        outDir,
        apiBase: 'https://api-dev.towncrierapp.uk',
        buildKey: 'test-key',
        fetchImpl: stub.fetch,
        loadAuthorities: async () => cornwallAuthorities,
        loadTowns: async () => cornwallTowns,
        logger: silentLogger,
      }),
    ).rejects.toThrow();
  });

  it('fails loud when the geo endpoint returns a non-OK status', async () => {
    const stub = new StubFetch(() => ({ ok: false, status: 503, body: null }));

    await expect(
      runPrerender({
        outDir,
        apiBase: 'https://api-dev.towncrierapp.uk',
        buildKey: 'test-key',
        fetchImpl: stub.fetch,
        loadAuthorities: async () => cornwallAuthorities,
        loadTowns: async () => cornwallTowns,
        logger: silentLogger,
      }),
    ).rejects.toThrow();
  });
});

// resolveMinPopulation (tc-2avw.3): the build-time published-population gate is a
// config value read from SEO_TOWN_MIN_POPULATION, defaulting to 20000 when the
// var is missing, empty, or not a positive finite integer.
describe('resolveMinPopulation', () => {
  it('defaults to 20000 when the env var is unset', () => {
    expect(resolveMinPopulation({})).toBe(20000);
  });

  it('defaults to 20000 when the env var is an empty string', () => {
    expect(resolveMinPopulation({ SEO_TOWN_MIN_POPULATION: '' })).toBe(20000);
  });

  it('defaults to 20000 when the env var is whitespace only', () => {
    expect(resolveMinPopulation({ SEO_TOWN_MIN_POPULATION: '   ' })).toBe(20000);
  });

  it('defaults to 20000 when the env var is non-numeric', () => {
    expect(resolveMinPopulation({ SEO_TOWN_MIN_POPULATION: 'lots' })).toBe(20000);
  });

  it('defaults to 20000 when the env var is zero or negative', () => {
    expect(resolveMinPopulation({ SEO_TOWN_MIN_POPULATION: '0' })).toBe(20000);
    expect(resolveMinPopulation({ SEO_TOWN_MIN_POPULATION: '-5000' })).toBe(20000);
  });

  it('reads a valid custom positive integer', () => {
    expect(resolveMinPopulation({ SEO_TOWN_MIN_POPULATION: '5000' })).toBe(5000);
  });

  it('truncates a fractional value to its integer part', () => {
    expect(resolveMinPopulation({ SEO_TOWN_MIN_POPULATION: '12345.9' })).toBe(
      12345,
    );
  });
});

// Population threshold filter (tc-2avw.3): the live prerender publishes a town
// only if its population >= the resolved threshold, applied BEFORE the per-town
// geo fetch so below-threshold towns never hit the API. The >=10 coverage gate
// still applies on top.
describe('runPrerender — population threshold filter (live mode)', () => {
  const cornwallAuthorities = [
    { id: 52, name: 'Cornwall', areaType: 'English County' },
  ];

  /** A geo stub that always clears the >=10 coverage gate. */
  function gatePassingStub() {
    return new StubFetch((url) => {
      if (url.includes('/v1/applications/near')) {
        return {
          ok: true,
          status: 200,
          body: {
            authorityId: 52,
            lat: 50.2632,
            lng: -5.051,
            radius: 5000,
            applications: [
              {
                uid: 'CW1',
                name: 'PA26/0001',
                address: 'Lemon Quay, Truro',
                description: 'Café conversion',
                appState: 'Permitted',
                startDate: '2026-01-12',
                lastDifferent: '2026-06-12T10:00:00+00:00',
                link: 'https://planit.org.uk/planapplic/CW1',
                url: null,
              },
            ],
            total: 14,
            statusBreakdown: [{ appState: 'Permitted', count: 14 }],
          },
        };
      }
      throw new Error(`unexpected url ${url}`);
    });
  }

  it('publishes a town above the default 20000 threshold', async () => {
    const stub = gatePassingStub();
    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      env: {},
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => [
        {
          slug: 'truro',
          name: 'Truro',
          lat: 50.2632,
          lng: -5.051,
          authorityId: 52,
          population: 25000,
        },
      ],
      logger: silentLogger,
    });

    expect(result.publishedTowns).toEqual(['cornwall/truro']);
  });

  it('publishes a town exactly at the threshold (>= is inclusive)', async () => {
    const stub = gatePassingStub();
    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      env: {},
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => [
        {
          slug: 'truro',
          name: 'Truro',
          lat: 50.2632,
          lng: -5.051,
          authorityId: 52,
          population: 20000,
        },
      ],
      logger: silentLogger,
    });

    expect(result.publishedTowns).toEqual(['cornwall/truro']);
  });

  it('excludes a below-threshold town with reason "population" and never fetches it', async () => {
    const stub = new StubFetch(() => {
      throw new Error('below-threshold town must not hit the geo endpoint');
    });

    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      env: {},
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => [
        {
          slug: 'truro',
          name: 'Truro',
          lat: 50.2632,
          lng: -5.051,
          authorityId: 52,
          population: 18766,
        },
      ],
      logger: silentLogger,
    });

    expect(result.publishedTowns).toEqual([]);
    expect(result.excludedTowns).toContainEqual({
      name: 'Truro',
      reason: 'population',
    });
    expect(await exists(join(outDir, 'planning', 'cornwall', 'truro'))).toBe(
      false,
    );
    // The whole point: no geo call is made for an excluded town.
    expect(stub.calls).toHaveLength(0);
  });

  it('a custom SEO_TOWN_MIN_POPULATION changes the cut', async () => {
    const stub = gatePassingStub();
    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      env: { SEO_TOWN_MIN_POPULATION: '5000' },
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => [
        {
          slug: 'truro',
          name: 'Truro',
          lat: 50.2632,
          lng: -5.051,
          authorityId: 52,
          // 18766 < default 20000 (would be excluded) but >= the custom 5000.
          population: 18766,
        },
      ],
      logger: silentLogger,
    });

    expect(result.publishedTowns).toEqual(['cornwall/truro']);
  });

  it('still excludes an above-threshold town that fails the coverage gate', async () => {
    const stub = new StubFetch(() => ({
      ok: true,
      status: 200,
      body: {
        authorityId: 52,
        lat: 50.2632,
        lng: -5.051,
        radius: 5000,
        applications: [],
        total: 3,
        statusBreakdown: [],
      },
    }));

    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      env: {},
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => [
        {
          slug: 'truro',
          name: 'Truro',
          lat: 50.2632,
          lng: -5.051,
          authorityId: 52,
          population: 25000,
        },
      ],
      logger: silentLogger,
    });

    expect(result.publishedTowns).toEqual([]);
    // It passed the population gate (so it WAS fetched) but the coverage gate
    // excluded it — reason "coverage", not "population".
    expect(result.excludedTowns).toContainEqual({
      name: 'Truro',
      reason: 'coverage',
    });
    expect(stub.calls).toHaveLength(1);
  });
});

// The integration spine for the per-town SEO pipeline (tc-2avw.2): drives ONE
// authority end-to-end through the two NEW pieces wired in this bead — the
// `population` field in the gazetteer schema and the `order=distance` near param —
// from gazetteer load → near fetch → coverage gate → page write → sitemap.
describe('runPrerender — per-town integration spine (one authority, end to end)', () => {
  const cornwallAuthorities = [
    { id: 52, name: 'Cornwall', areaType: 'English County' },
  ];

  it('refuses a gazetteer whose town row has a malformed population', async () => {
    const readImpl = async () =>
      JSON.stringify([
        {
          slug: 'truro',
          name: 'Truro',
          lat: 50.2632,
          lng: -5.051,
          authorityId: 52,
          population: 'lots',
        },
      ]);
    await expect(loadTownsFromFile('/fake/towns.json', readImpl)).rejects.toThrow();
  });

  it('loads a population-bearing gazetteer, requests order=distance, gates, writes the page, and lists it in the sitemap', async () => {
    // 1) Gazetteer load: a real loadTownsFromFile round-trip carrying `population`.
    const gazetteer = async () =>
      JSON.stringify([
        {
          slug: 'truro',
          name: 'Truro',
          lat: 50.2632,
          lng: -5.051,
          authorityId: 52,
          population: 18766,
        },
      ]);
    const towns = await loadTownsFromFile('/fake/towns.json', gazetteer);
    expect(towns[0].population).toBe(18766);

    // 2) Near fetch: hand-written fake fetch, total=14 (clears the >=10 gate).
    const stub = new StubFetch((url) => {
      if (url.includes('/v1/applications/near')) {
        return {
          ok: true,
          status: 200,
          body: {
            authorityId: 52,
            lat: 50.2632,
            lng: -5.051,
            radius: 5000,
            applications: [
              {
                uid: 'CW1',
                name: 'PA26/0001',
                address: 'Lemon Quay, Truro',
                description: 'Café conversion',
                appState: 'Permitted',
                startDate: '2026-01-12',
                lastDifferent: '2026-06-12T10:00:00+00:00',
                link: 'https://planit.org.uk/planapplic/CW1',
                url: null,
              },
            ],
            total: 14,
            statusBreakdown: [{ appState: 'Permitted', count: 14 }],
          },
        };
      }
      throw new Error(`unexpected url ${url}`);
    });

    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => towns,
      logger: silentLogger,
    });

    // 3) order=distance: the near request consumes the tc-2avw.1 param.
    const nearCalls = stub.calls.filter((c) =>
      c.url.includes('/v1/applications/near'),
    );
    expect(nearCalls).toHaveLength(1);
    expect(nearCalls[0].url).toContain('order=distance');

    // No PlanIt at build time — only our own build-key-gated API.
    expect(stub.calls.every((c) => !c.url.includes('planit.org.uk'))).toBe(true);

    // 4) Coverage gate cleared -> the nested town page is published and written.
    expect(result.publishedTowns).toEqual(['cornwall/truro']);
    const html = await readFile(
      join(outDir, 'planning', 'cornwall', 'truro', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<h1>Planning applications in Truro</h1>');

    // 5) Sitemap carries the published town path.
    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/cornwall/truro</loc>',
    );
  });
});
