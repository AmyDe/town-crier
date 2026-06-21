import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, readFile, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { runPrerender } from '../../prerender-planning.mjs';

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
  const cornwallAuthorities = [
    { id: 52, name: 'Cornwall', areaType: 'English Unitary Authority' },
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
                link: 'https://planit.org.uk/planapplic/CW1',
                url: null,
              },
            ],
            total: 14,
            totalCapped: false,
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
        totalCapped: false,
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
