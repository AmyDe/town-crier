import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, readFile, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { runFetch, runRender } from '../../prerender-planning.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const SNAPSHOT_FIXTURE = join(
  here,
  '..',
  '..',
  'fixtures',
  'sample-snapshot.json',
);

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
  outDir = await mkdtemp(join(tmpdir(), 'prerender-snapshot-'));
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

/**
 * Run a body with `globalThis.fetch` replaced by a tripwire that records every
 * call and throws if invoked. Render mode must be provably network-free, so any
 * call at all is a failure. Returns the recorded calls so the caller can assert
 * zero invocations even when the body never throws.
 */
async function withFetchTripwire(body) {
  const calls = [];
  const original = globalThis.fetch;
  globalThis.fetch = (...args) => {
    calls.push(args);
    throw new Error('render mode must not perform any network I/O');
  };
  try {
    await body();
  } finally {
    globalThis.fetch = original;
  }
  return calls;
}

// --fetch (snapshot mode): the same live data-gathering as runLiveMode (all
// authorities + population-eligible towns via the build-key-gated API), but
// serialised into a single self-contained seo-snapshot.json instead of HTML.
describe('runFetch — snapshot mode', () => {
  const authorities = [
    { id: 1, name: 'Adur', areaType: 'English District' }, // qualifying
    { id: 2, name: 'West Sussex', areaType: 'English County' }, // non-qualifying
    { id: 52, name: 'Cornwall', areaType: 'English County' }, // non-qualifying, parent of Truro
  ];
  const towns = [
    {
      slug: 'truro',
      name: 'Truro',
      lat: 50.2632,
      lng: -5.051,
      authorityId: 52,
      population: 25000, // above default 20000 threshold -> eligible
    },
    {
      slug: 'tiny',
      name: 'Tiny',
      lat: 50.1,
      lng: -5.1,
      authorityId: 52,
      population: 5000, // below threshold -> excluded, never fetched
    },
  ];

  /** A stub that answers both the authority and the geo endpoints. */
  function fullStub() {
    return new StubFetch((url) => {
      if (url.includes('/v1/authorities/1/applications')) {
        return {
          ok: true,
          status: 200,
          body: {
            authorityId: 1,
            areaName: 'Adur',
            applications: [
              {
                uid: 'A1',
                name: '26/0001',
                address: '1 Sea Road',
                description: 'Extension',
                appState: 'Permitted',
                startDate: '2026-01-10',
                lastDifferent: '2026-06-10T10:00:00+00:00',
                link: 'https://planit.org.uk/planapplic/A1',
                url: null,
              },
            ],
            total: 12,
            statusBreakdown: [{ appState: 'Permitted', count: 12 }],
          },
        };
      }
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

  it('serialises every qualifying authority + eligible town + the authority list, and emits no HTML', async () => {
    const stub = fullStub();
    const snapshotPath = join(outDir, 'seo-snapshot.json');

    const snapshot = await runFetch({
      snapshotPath,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      env: {},
      loadAuthorities: async () => authorities,
      loadTowns: async () => towns,
      logger: silentLogger,
    });

    // The snapshot is written to disk and round-trips through JSON.
    const onDisk = JSON.parse(await readFile(snapshotPath, 'utf-8'));
    expect(onDisk).toEqual(snapshot);

    // Every QUALIFYING authority is carried with its full render inputs; the two
    // English Counties are dropped by the areaType pre-filter (same as live mode).
    expect(snapshot.authorityPages.map((a) => a.name)).toEqual(['Adur']);
    const adur = snapshot.authorityPages[0];
    expect(adur).toMatchObject({
      id: 1,
      name: 'Adur',
      areaType: 'English District',
      areaName: 'Adur',
      total: 12,
    });
    expect(adur.statusBreakdown).toEqual([{ appState: 'Permitted', count: 12 }]);
    expect(adur.applications).toHaveLength(1);

    // Every ELIGIBLE town is carried with its full render inputs; the below-
    // threshold town is dropped by the population pre-filter (same as live mode).
    expect(snapshot.townPages.map((t) => t.slug)).toEqual(['truro']);
    const truro = snapshot.townPages[0];
    expect(truro).toMatchObject({
      slug: 'truro',
      name: 'Truro',
      lat: 50.2632,
      lng: -5.051,
      authorityId: 52,
      population: 25000,
      total: 14,
    });
    expect(truro.statusBreakdown).toEqual([{ appState: 'Permitted', count: 14 }]);
    expect(truro.applications).toHaveLength(1);

    // The FULL authority list is embedded so a town's parent-authority slug can
    // be resolved offline at render time — including non-qualifying parents
    // (Cornwall) that never get an authority page of their own.
    expect(snapshot.authorities).toEqual([
      { id: 1, name: 'Adur' },
      { id: 2, name: 'West Sussex' },
      { id: 52, name: 'Cornwall' },
    ]);

    // Metadata makes the snapshot self-contained / self-describing.
    expect(snapshot.minPopulation).toBe(20000);
    expect(snapshot.limit).toBe(30);
    expect(typeof snapshot.generatedAt).toBe('string');

    // Emits NO HTML and NO sitemap — only the snapshot file.
    expect(await exists(join(outDir, 'planning'))).toBe(false);
    expect(await exists(join(outDir, 'sitemap.xml'))).toBe(false);
  });

  it('fetches only the gated endpoints with X-Build-Key and never fetches a below-threshold town', async () => {
    const stub = fullStub();

    await runFetch({
      snapshotPath: join(outDir, 'seo-snapshot.json'),
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      env: {},
      loadAuthorities: async () => authorities,
      loadTowns: async () => towns,
      logger: silentLogger,
    });

    // West Sussex (English County) is never fetched: exactly one authority call.
    const appsCalls = stub.calls.filter((c) =>
      c.url.includes('/v1/authorities/'),
    );
    expect(appsCalls).toHaveLength(1);
    expect(appsCalls[0].url).toContain('/v1/authorities/1/applications');
    expect(appsCalls[0].init.headers['X-Build-Key']).toBe('test-key');

    // The below-threshold "Tiny" town never hits the geo endpoint: exactly one
    // near call (for Truro), carrying order=distance and the build key.
    const nearCalls = stub.calls.filter((c) =>
      c.url.includes('/v1/applications/near'),
    );
    expect(nearCalls).toHaveLength(1);
    expect(nearCalls[0].url).toContain('authorityId=52');
    expect(nearCalls[0].url).toContain('order=distance');
    expect(nearCalls[0].init.headers['X-Build-Key']).toBe('test-key');

    // Never PlanIt — only our own API.
    expect(stub.calls.every((c) => !c.url.includes('planit.org.uk'))).toBe(true);
  });

  it('honours a custom SEO_TOWN_MIN_POPULATION cut', async () => {
    const stub = fullStub();
    const snapshot = await runFetch({
      snapshotPath: join(outDir, 'seo-snapshot.json'),
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      env: { SEO_TOWN_MIN_POPULATION: '1000' },
      loadAuthorities: async () => authorities,
      loadTowns: async () => towns,
      logger: silentLogger,
    });

    // With the cut lowered to 1000, the previously-excluded "Tiny" town qualifies.
    expect(snapshot.townPages.map((t) => t.slug).sort()).toEqual([
      'tiny',
      'truro',
    ]);
    expect(snapshot.minPopulation).toBe(1000);
  });

  it('fails loud when the applications endpoint returns an unexpected shape', async () => {
    const stub = new StubFetch(() => ({
      ok: true,
      status: 200,
      body: { not: 'what we expect' },
    }));

    await expect(
      runFetch({
        snapshotPath: join(outDir, 'seo-snapshot.json'),
        apiBase: 'https://api-dev.towncrierapp.uk',
        buildKey: 'test-key',
        fetchImpl: stub.fetch,
        env: {},
        loadAuthorities: async () => [
          { id: 1, name: 'Adur', areaType: 'English District' },
        ],
        loadTowns: async () => [],
        logger: silentLogger,
      }),
    ).rejects.toThrow();
  });

  it('fails loud when no build key is set', async () => {
    await expect(
      runFetch({
        snapshotPath: join(outDir, 'seo-snapshot.json'),
        apiBase: 'https://api-dev.towncrierapp.uk',
        buildKey: undefined,
        loadAuthorities: async () => authorities,
        loadTowns: async () => towns,
        logger: silentLogger,
      }),
    ).rejects.toThrow();
  });

  it('fails loud when no API base is configured', async () => {
    await expect(
      runFetch({
        snapshotPath: join(outDir, 'seo-snapshot.json'),
        apiBase: undefined,
        buildKey: 'test-key',
        loadAuthorities: async () => authorities,
        loadTowns: async () => towns,
        logger: silentLogger,
      }),
    ).rejects.toThrow();
  });
});

// --render (offline mode): read seo-snapshot.json from disk and render all
// /planning/* pages + sitemap.xml with ZERO network calls.
describe('runRender — offline mode', () => {
  it('reproduces the authority + town page set and sitemap from a fixture snapshot with no network', async () => {
    let result;
    const fetchCalls = await withFetchTripwire(async () => {
      result = await runRender({
        outDir,
        snapshotPath: SNAPSHOT_FIXTURE,
        logger: silentLogger,
      });
    });

    // Provably network-free: the global fetch tripwire was never invoked.
    expect(fetchCalls).toHaveLength(0);

    expect(result.skipped).toBe(false);
    expect(result.published).toEqual(['basingstoke-and-deane']);
    expect(result.publishedTowns).toEqual(['cornwall/truro']);

    // The authority page renders, identical to live/fixture output.
    const authorityHtml = await readFile(
      join(outDir, 'planning', 'basingstoke-and-deane', 'index.html'),
      'utf-8',
    );
    expect(authorityHtml).toContain(
      '<h1>Planning applications in Basingstoke and Deane</h1>',
    );
    expect(authorityHtml).toContain('Data updated 15 Jun 2026');

    // The nested town page renders, with the parent-authority slug resolved
    // offline from the embedded authority list (Cornwall id 52 -> "cornwall").
    const townHtml = await readFile(
      join(outDir, 'planning', 'cornwall', 'truro', 'index.html'),
      'utf-8',
    );
    expect(townHtml).toContain('<h1>Planning applications in Truro</h1>');
    expect(townHtml).toContain(
      '<link rel="canonical" href="https://towncrierapp.uk/planning/cornwall/truro"',
    );

    // The sitemap lists both pages with content-derived lastmods.
    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/basingstoke-and-deane</loc>',
    );
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/cornwall/truro</loc>',
    );
    expect(sitemap).toContain('<lastmod>2026-06-15</lastmod>');
    expect(sitemap).toContain('<lastmod>2026-06-14</lastmod>');
  });

  it('links an authority page down to its published town children, sorted by name', async () => {
    const { writeFile } = await import('node:fs/promises');
    const snapshotPath = join(outDir, 'wiring-snapshot.json');
    const app = (uid) => ({
      uid,
      name: uid,
      address: `${uid} address`,
      description: 'desc',
      appState: 'Permitted',
      startDate: '2026-01-10',
      lastDifferent: '2026-06-10T10:00:00+00:00',
      link: null,
      url: null,
    });
    await writeFile(
      snapshotPath,
      JSON.stringify({
        version: 1,
        generatedAt: '2026-06-25T00:00:00.000Z',
        minPopulation: 20000,
        limit: 30,
        authorities: [{ id: 1, name: 'Adur' }],
        authorityPages: [
          {
            id: 1,
            name: 'Adur',
            areaType: 'English District',
            areaName: 'Adur',
            total: 12,
            statusBreakdown: [{ appState: 'Permitted', count: 12 }],
            applications: [app('A1')],
          },
        ],
        townPages: [
          {
            slug: 'shoreham-by-sea',
            name: 'Shoreham-by-Sea',
            lat: 50.83,
            lng: -0.27,
            authorityId: 1,
            population: 25000,
            total: 14,
            statusBreakdown: [{ appState: 'Permitted', count: 14 }],
            applications: [app('S1')],
          },
          {
            slug: 'lancing',
            name: 'Lancing',
            lat: 50.83,
            lng: -0.32,
            authorityId: 1,
            population: 30000,
            total: 11,
            statusBreakdown: [{ appState: 'Permitted', count: 11 }],
            applications: [app('L1')],
          },
        ],
      }),
      'utf-8',
    );

    const result = await runRender({
      outDir,
      snapshotPath,
      logger: silentLogger,
    });

    expect(result.published).toEqual(['adur']);

    const html = await readFile(
      join(outDir, 'planning', 'adur', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<section class="townLinks">');
    expect(html).toContain('<a href="/planning/adur/lancing">Lancing</a>');
    expect(html).toContain(
      '<a href="/planning/adur/shoreham-by-sea">Shoreham-by-Sea</a>',
    );
    expect(html.indexOf('>Lancing<')).toBeLessThan(
      html.indexOf('>Shoreham-by-Sea<'),
    );
  });

  it('fails loud when the snapshot file is missing', async () => {
    await expect(
      runRender({
        outDir,
        snapshotPath: join(outDir, 'does-not-exist.json'),
        logger: silentLogger,
      }),
    ).rejects.toThrow();
  });

  it('fails loud when the snapshot is malformed JSON', async () => {
    const badPath = join(outDir, 'bad.json');
    const { writeFile } = await import('node:fs/promises');
    await writeFile(badPath, 'not json at all', 'utf-8');

    await expect(
      runRender({ outDir, snapshotPath: badPath, logger: silentLogger }),
    ).rejects.toThrow();
  });

  it('fails loud when the snapshot is missing a required section', async () => {
    const badPath = join(outDir, 'incomplete.json');
    const { writeFile } = await import('node:fs/promises');
    // No townPages section -> not a valid snapshot.
    await writeFile(
      badPath,
      JSON.stringify({ authorities: [], authorityPages: [] }),
      'utf-8',
    );

    await expect(
      runRender({ outDir, snapshotPath: badPath, logger: silentLogger }),
    ).rejects.toThrow();
  });
});

// The whole point of the split: a snapshot produced by --fetch must render the
// SAME page set + sitemap offline, with zero network.
describe('fetch -> render round trip', () => {
  it('renders the same pages and sitemap offline from a freshly-fetched snapshot', async () => {
    const snapshotPath = join(outDir, 'seo-snapshot.json');

    const stub = new StubFetch((url) => {
      if (url.includes('/v1/authorities/1/applications')) {
        return {
          ok: true,
          status: 200,
          body: {
            authorityId: 1,
            areaName: 'Adur',
            applications: [
              {
                uid: 'A1',
                name: '26/0001',
                address: '1 Sea Road',
                description: 'Extension',
                appState: 'Permitted',
                startDate: '2026-01-10',
                lastDifferent: '2026-06-10T10:00:00+00:00',
                link: 'https://planit.org.uk/planapplic/A1',
                url: null,
              },
            ],
            total: 12,
            statusBreakdown: [{ appState: 'Permitted', count: 12 }],
          },
        };
      }
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

    await runFetch({
      snapshotPath,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      env: {},
      loadAuthorities: async () => [
        { id: 1, name: 'Adur', areaType: 'English District' },
        { id: 52, name: 'Cornwall', areaType: 'English County' },
      ],
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

    let result;
    const fetchCalls = await withFetchTripwire(async () => {
      result = await runRender({
        outDir,
        snapshotPath,
        logger: silentLogger,
      });
    });

    expect(fetchCalls).toHaveLength(0);
    expect(result.published).toEqual(['adur']);
    expect(result.publishedTowns).toEqual(['cornwall/truro']);

    expect(
      await exists(join(outDir, 'planning', 'adur', 'index.html')),
    ).toBe(true);
    expect(
      await exists(join(outDir, 'planning', 'cornwall', 'truro', 'index.html')),
    ).toBe(true);

    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    expect(sitemap).toContain('<loc>https://towncrierapp.uk/planning/adur</loc>');
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/cornwall/truro</loc>',
    );
  });
});
