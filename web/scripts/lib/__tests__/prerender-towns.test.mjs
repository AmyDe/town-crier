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

  it('stamps the town sitemap <lastmod> with the max lastDifferent of its shown applications', async () => {
    await runPrerender({
      outDir,
      townFixturePath: TOWN_FIXTURE,
      logger: silentLogger,
    });

    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    // Truro's shown apps last-change on 14 Jun and 11 Jun 2026; the page's
    // lastmod is the freshest of those (14 Jun), not the build clock.
    expect(sitemap).toContain('<lastmod>2026-06-14</lastmod>');
    expect(sitemap).not.toContain('<lastmod>2026-06-11</lastmod>');
    expect(sitemap).not.toContain('<lastmod></lastmod>');
  });
});

describe('runPrerender — town live mode', () => {
  // population well above the default 20000 threshold so these tests isolate the
  // geo/coverage pipeline (tc-2avw.3's population gate is exercised separately).
  const cornwallTowns = [
    {
      slug: 'truro',
      name: 'Truro',
      lat: 50.2632,
      lng: -5.051,
      authorityId: 52,
      population: 25000,
    },
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
    // tc-s0yf: the server now always returns town-partitioned, pre-ordered
    // results — order=distance is gone, and the primary point carries an
    // explicit radius (mirroring the server's own 5000m default).
    expect(nearCalls[0].url).not.toContain('order=distance');
    expect(nearCalls[0].url).toContain('radius=5000');
    // Truro is the only gazetteer town in this authority, so no sibling
    // centroid is sent at all.
    expect(nearCalls[0].url).not.toContain('sibling=');
    expect(nearCalls[0].init.headers['X-Build-Key']).toBe('test-key');

    const html = await readFile(
      join(outDir, 'planning', 'cornwall', 'truro', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<h1>Planning applications in Truro</h1>');
  });

  it('renders the applications in the exact order the API returned them (no client-side re-sort, tc-s0yf)', async () => {
    // The near read is now fully server-ordered (town-level Voronoi partition +
    // GREATEST(decidedDate, startDate) DESC, tc-s0yf) — replacing the old
    // order=distance + client re-sort-by-lastDifferent pipeline. This fixture
    // deliberately puts the OLDER-lastDifferent app FIRST in the API response and
    // the NEWER-lastDifferent app SECOND: if the old client-side recency re-sort
    // were still running, it would flip them. It must not — the page must render
    // the API's given order as-is, proving the re-sort is truly gone (not just
    // reordered to a no-op).
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
                uid: 'CW-FIRST',
                name: 'PA26/FIRST',
                address: 'First In API Order, Truro',
                description: 'Server-ordered first, despite an OLDER lastDifferent',
                appState: 'Permitted',
                startDate: '2026-01-01',
                decidedDate: null,
                lastDifferent: '2026-06-01T08:00:00+00:00',
                link: 'https://planit.org.uk/planapplic/CW-FIRST',
                url: null,
              },
              {
                uid: 'CW-SECOND',
                name: 'PA26/SECOND',
                address: 'Second In API Order, Truro',
                description: 'Server-ordered second, despite a NEWER lastDifferent',
                appState: 'Rejected',
                startDate: '2026-02-02',
                decidedDate: '2026-06-20',
                lastDifferent: '2026-06-20T09:00:00+00:00',
                link: 'https://planit.org.uk/planapplic/CW-SECOND',
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

    await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => cornwallTowns,
      logger: silentLogger,
    });

    const html = await readFile(
      join(outDir, 'planning', 'cornwall', 'truro', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('First In API Order, Truro');
    expect(html).toContain('Second In API Order, Truro');
    expect(html.indexOf('First In API Order, Truro')).toBeLessThan(
      html.indexOf('Second In API Order, Truro'),
    );
    // The page-level "Data updated" line is unaffected by render order — still
    // the freshest lastDifferent among the shown set.
    expect(html).toContain('Data updated 20 Jun 2026');

    // The sitemap lastmod is likewise the freshest shown app's lastDifferent,
    // independent of render order.
    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    expect(sitemap).toContain('<lastmod>2026-06-20</lastmod>');
  });

  it('omits the town sitemap <lastmod> when no shown application carries a lastDifferent date', async () => {
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
            // 12 undated apps: clears the >=10 gate but has no date to stamp.
            applications: Array.from({ length: 12 }, (_, i) => ({
              uid: `U${i}`,
              name: `PA26/${i}`,
              address: `Addr ${i}, Truro`,
              description: 'Undated',
              appState: 'Permitted',
              startDate: null,
              lastDifferent: null,
              link: null,
              url: null,
            })),
            total: 12,
            statusBreakdown: [{ appState: 'Permitted', count: 12 }],
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

    // The page IS published (cleared the gate) ...
    expect(result.publishedTowns).toEqual(['cornwall/truro']);
    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/cornwall/truro</loc>',
    );
    // ... but carries no lastmod — better than an invalid/empty one.
    expect(sitemap).not.toContain('<lastmod>');
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

// Sibling centroid passthrough (tc-s0yf.2, GH #819): the near query now carries
// one `sibling=lat,lng,radius` per OTHER gazetteer town in the same authority,
// so the server can run its town-level Voronoi partition. All radii — primary
// and sibling alike — use the same build-time default (5000m, mirroring the
// server's own default) since the committed gazetteer has no per-town radius.
describe('runPrerender — town live mode: sibling centroid passthrough (tc-s0yf.2)', () => {
  const cornwallAuthorities = [
    { id: 52, name: 'Cornwall', areaType: 'English County' },
  ];

  /** A geo stub that always clears the >=10 coverage gate for any town. */
  function gatePassingStub() {
    return new StubFetch((url) => {
      if (url.includes('/v1/applications/near')) {
        return {
          ok: true,
          status: 200,
          body: {
            authorityId: 52,
            radius: 5000,
            applications: [
              {
                uid: 'CW1',
                name: 'PA26/0001',
                address: 'Some address',
                description: 'Some description',
                appState: 'Permitted',
                startDate: '2026-01-12',
                lastDifferent: '2026-06-12T10:00:00+00:00',
                link: null,
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

  it('serializes every OTHER same-authority gazetteer town as a repeated sibling=lat,lng,radius param', async () => {
    const truro = {
      slug: 'truro',
      name: 'Truro',
      lat: 50.2632,
      lng: -5.051,
      authorityId: 52,
      population: 25000,
    };
    const falmouth = {
      slug: 'falmouth',
      name: 'Falmouth',
      lat: 50.1526,
      lng: -5.0745,
      authorityId: 52,
      population: 22000,
    };
    const newquay = {
      slug: 'newquay',
      name: 'Newquay',
      lat: 50.4155,
      lng: -5.0904,
      authorityId: 52,
      population: 21000,
    };
    const stub = gatePassingStub();

    await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => [truro, falmouth, newquay],
      logger: silentLogger,
    });

    const nearCalls = stub.calls.filter((c) => c.url.includes('/v1/applications/near'));
    expect(nearCalls).toHaveLength(3);

    const truroCall = nearCalls.find((c) => c.url.includes('lat=50.2632'));
    expect(truroCall).toBeDefined();
    // Truro's siblings are Falmouth + Newquay (NOT Truro itself) — exactly two.
    expect(truroCall.url).toContain('sibling=50.1526,-5.0745,5000');
    expect(truroCall.url).toContain('sibling=50.4155,-5.0904,5000');
    expect((truroCall.url.match(/sibling=/g) ?? [])).toHaveLength(2);
    expect(truroCall.url).not.toContain('sibling=50.2632,-5.051,5000');
  });

  it('omits the sibling param entirely when the town is the only gazetteer town in its authority', async () => {
    const soleTown = {
      slug: 'truro',
      name: 'Truro',
      lat: 50.2632,
      lng: -5.051,
      authorityId: 52,
      population: 25000,
    };
    const stub = gatePassingStub();

    await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => [soleTown],
      logger: silentLogger,
    });

    const nearCalls = stub.calls.filter((c) => c.url.includes('/v1/applications/near'));
    expect(nearCalls).toHaveLength(1);
    expect(nearCalls[0].url).not.toContain('sibling=');
  });

  it('still passes a below-population-threshold town as a sibling centroid, even though it is never itself fetched', async () => {
    const truro = {
      slug: 'truro',
      name: 'Truro',
      lat: 50.2632,
      lng: -5.051,
      authorityId: 52,
      population: 25000,
    };
    // Below the default 20000 threshold: never gets its own near fetch, but its
    // gazetteer centroid still contributes to the Voronoi partition (decision 2,
    // GH #819) — all gazetteer towns in an authority are sibling candidates.
    const tiny = {
      slug: 'tiny',
      name: 'Tiny',
      lat: 50.3,
      lng: -5.2,
      authorityId: 52,
      population: 5000,
    };
    const stub = gatePassingStub();

    await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      env: {},
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => [truro, tiny],
      logger: silentLogger,
    });

    // Tiny never gets its own near call (below threshold)...
    const nearCalls = stub.calls.filter((c) => c.url.includes('/v1/applications/near'));
    expect(nearCalls).toHaveLength(1);
    // ...but its centroid IS sent as Truro's sibling.
    expect(nearCalls[0].url).toContain('sibling=50.3,-5.2,5000');
  });

  it('never sends siblings for towns in a DIFFERENT authority', async () => {
    const truro = {
      slug: 'truro',
      name: 'Truro',
      lat: 50.2632,
      lng: -5.051,
      authorityId: 52,
      population: 25000,
    };
    const otherAuthorityTown = {
      slug: 'elsewhere',
      name: 'Elsewhere',
      lat: 51.5,
      lng: -0.1,
      authorityId: 999,
      population: 30000,
    };
    const stub = new StubFetch((url) => {
      if (url.includes('/v1/applications/near')) {
        return {
          ok: true,
          status: 200,
          body: {
            authorityId: url.includes('authorityId=52') ? 52 : 999,
            radius: 5000,
            applications: [],
            total: 14,
            statusBreakdown: [],
          },
        };
      }
      throw new Error(`unexpected url ${url}`);
    });

    await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      loadAuthorities: async () => [
        ...cornwallAuthorities,
        // Non-qualifying areaType (as cornwallAuthorities already is), so this
        // test isolates the TOWN/near pipeline and never hits the authority
        // applications endpoint.
        { id: 999, name: 'Elsewhere Council', areaType: 'English County' },
      ],
      loadTowns: async () => [truro, otherAuthorityTown],
      logger: silentLogger,
    });

    const nearCalls = stub.calls.filter((c) => c.url.includes('/v1/applications/near'));
    expect(nearCalls).toHaveLength(2);
    for (const call of nearCalls) {
      expect(call.url).not.toContain('sibling=');
    }
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
// `population` field in the gazetteer schema and the near-query radius param
// (order=distance is gone — tc-s0yf) — from gazetteer load → near fetch →
// coverage gate → page write → sitemap.
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

  it('loads a population-bearing gazetteer, requests a radius-bounded near read, gates, writes the page, and lists it in the sitemap', async () => {
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
      // Admit the pop-18766 spine town past tc-2avw.3's population gate (default
      // 20000) so this end-to-end test still exercises fetch -> gate -> write.
      env: { SEO_TOWN_MIN_POPULATION: '5000' },
      loadAuthorities: async () => cornwallAuthorities,
      loadTowns: async () => towns,
      logger: silentLogger,
    });

    // 3) The near request is radius-bounded and no longer requests order=distance
    // (tc-s0yf); a lone town in its authority sends no sibling param.
    const nearCalls = stub.calls.filter((c) =>
      c.url.includes('/v1/applications/near'),
    );
    expect(nearCalls).toHaveLength(1);
    expect(nearCalls[0].url).not.toContain('order=distance');
    expect(nearCalls[0].url).toContain('radius=5000');
    expect(nearCalls[0].url).not.toContain('sibling=');

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
