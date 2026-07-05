import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, readFile, writeFile, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { runPrerender } from '../../prerender-planning.mjs';

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
  outDir = await mkdtemp(join(tmpdir(), 'prerender-towns-index-'));
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

describe('runPrerender — town index page (GH #821 Phase 2)', () => {
  async function writeFixtures() {
    const authorityFixture = join(outDir, 'authorities.json');
    const townFixture = join(outDir, 'towns.json');

    await writeFile(
      authorityFixture,
      JSON.stringify([
        {
          id: 1,
          name: 'Adur',
          areaType: 'English District',
          areaName: 'Adur',
          total: 12,
          statusBreakdown: [{ appState: 'Permitted', count: 12 }],
          applications: [app('A1')],
        },
        {
          id: 52,
          name: 'Cornwall',
          areaType: 'English Unitary Authority',
          areaName: 'Cornwall',
          total: 20,
          statusBreakdown: [{ appState: 'Permitted', count: 20 }],
          applications: [app('C1')],
        },
      ]),
      'utf-8',
    );

    await writeFile(
      townFixture,
      JSON.stringify([
        {
          slug: 'lancing',
          name: 'Lancing',
          lat: 50.83,
          lng: -0.32,
          authorityId: 1,
          total: 11,
          statusBreakdown: [{ appState: 'Permitted', count: 11 }],
          applications: [app('L1')],
        },
        {
          slug: 'truro',
          name: 'Truro',
          lat: 50.26,
          lng: -5.05,
          authorityId: 52,
          total: 18,
          statusBreakdown: [{ appState: 'Permitted', count: 18 }],
          applications: [app('T1')],
        },
        {
          slug: 'tiny',
          name: 'Tiny',
          lat: 50.8,
          lng: -0.3,
          authorityId: 1,
          total: 4, // below the coverage gate -> never published, never listed
          statusBreakdown: [{ appState: 'Permitted', count: 4 }],
          applications: [],
        },
      ]),
      'utf-8',
    );

    return { authorityFixture, townFixture };
  }

  it('writes a real index.html at planning/towns/index.html listing every published town A-Z with authority context', async () => {
    const { authorityFixture, townFixture } = await writeFixtures();

    await runPrerender({
      outDir,
      fixturePath: authorityFixture,
      townFixturePath: townFixture,
      loadAuthorities: async () => [
        { id: 1, name: 'Adur', areaType: 'English District' },
        { id: 52, name: 'Cornwall', areaType: 'English Unitary Authority' },
      ],
      logger: silentLogger,
    });

    const html = await readFile(
      join(outDir, 'planning', 'towns', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<h1>Planning applications by town</h1>');
    expect(html).toContain('<a href="/planning/adur/lancing">Lancing</a>');
    expect(html).toContain('<a href="/planning/cornwall/truro">Truro</a>');
    // Authority name shown as visible context alongside each town.
    expect(html).toMatch(/Lancing<\/a>[\s\S]{0,80}Adur/);
    expect(html).toMatch(/Truro<\/a>[\s\S]{0,80}Cornwall/);
    // A-Z ordering: Lancing (L) before Truro (T).
    expect(html.indexOf('>Lancing<')).toBeLessThan(html.indexOf('>Truro<'));
    // The gated-out town is never listed.
    expect(html).not.toContain('>Tiny<');
  });

  it('adds a /planning/towns sitemap entry alongside the authority and town pages', async () => {
    const { authorityFixture, townFixture } = await writeFixtures();

    await runPrerender({
      outDir,
      fixturePath: authorityFixture,
      townFixturePath: townFixture,
      loadAuthorities: async () => [
        { id: 1, name: 'Adur', areaType: 'English District' },
        { id: 52, name: 'Cornwall', areaType: 'English Unitary Authority' },
      ],
      logger: silentLogger,
    });

    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/towns</loc>',
    );
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/adur/lancing</loc>',
    );
  });

  it('still writes planning/towns/index.html when there are zero published towns', async () => {
    const authorityFixture = join(outDir, 'authorities-only.json');
    await writeFile(
      authorityFixture,
      JSON.stringify([
        {
          id: 1,
          name: 'Adur',
          areaType: 'English District',
          areaName: 'Adur',
          total: 12,
          statusBreakdown: [{ appState: 'Permitted', count: 12 }],
          applications: [app('A1')],
        },
      ]),
      'utf-8',
    );

    await runPrerender({
      outDir,
      fixturePath: authorityFixture,
      loadAuthorities: async () => [{ id: 1, name: 'Adur' }],
      logger: silentLogger,
    });

    expect(await exists(join(outDir, 'planning', 'towns', 'index.html'))).toBe(
      true,
    );
  });

  it('fails loudly when an authority name in the fixture would collide with the /planning/towns route', async () => {
    const authorityFixture = join(outDir, 'colliding-authority.json');
    await writeFile(
      authorityFixture,
      JSON.stringify([
        {
          id: 1,
          name: 'Adur',
          areaType: 'English District',
          areaName: 'Adur',
          total: 12,
          statusBreakdown: [{ appState: 'Permitted', count: 12 }],
          applications: [app('A1')],
        },
        {
          id: 2,
          name: 'Towns',
          areaType: 'English District',
          areaName: 'Towns',
          total: 12,
          statusBreakdown: [{ appState: 'Permitted', count: 12 }],
          applications: [app('B1')],
        },
      ]),
      'utf-8',
    );

    await expect(
      runPrerender({
        outDir,
        fixturePath: authorityFixture,
        loadAuthorities: async () => [{ id: 1, name: 'Adur' }],
        logger: silentLogger,
      }),
    ).rejects.toThrow(/towns/i);
  });

  it('fails loudly in live mode when the committed authority list has a name colliding with /planning/towns', async () => {
    const stub = new StubFetch(() => {
      throw new Error('applications endpoint must not be called');
    });

    await expect(
      runPrerender({
        outDir,
        apiBase: 'https://api-dev.towncrierapp.uk',
        buildKey: 'test-key',
        fetchImpl: stub.fetch,
        loadAuthorities: async () => [
          { id: 1, name: 'Adur', areaType: 'English District' },
          { id: 2, name: 'Towns', areaType: 'English District' },
        ],
        loadTowns: async () => [],
        logger: silentLogger,
      }),
    ).rejects.toThrow(/towns/i);
    // Fails before any HTTP call — the guard runs first.
    expect(stub.calls).toHaveLength(0);
  });

  it('renders the town index from live-fetched geo data (town live mode)', async () => {
    const stub = new StubFetch((url) => {
      if (url.includes('/v1/applications/near')) {
        return {
          ok: true,
          status: 200,
          body: {
            applications: [app('T1')],
            total: 14,
            statusBreakdown: [{ appState: 'Permitted', count: 14 }],
          },
        };
      }
      if (url.includes('/v1/authorities/52/applications')) {
        return {
          ok: true,
          status: 200,
          body: {
            areaName: 'Cornwall',
            applications: [app('C1')],
            total: 20,
            statusBreakdown: [{ appState: 'Permitted', count: 20 }],
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
        { id: 52, name: 'Cornwall', areaType: 'English Unitary Authority' },
      ],
      loadTowns: async () => [
        {
          slug: 'truro',
          name: 'Truro',
          lat: 50.26,
          lng: -5.05,
          authorityId: 52,
          population: 25000,
        },
      ],
      logger: silentLogger,
    });

    const html = await readFile(
      join(outDir, 'planning', 'towns', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<a href="/planning/cornwall/truro">Truro</a>');
  });
});
