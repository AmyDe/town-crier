import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, readFile, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { runPrerender } from '../../prerender-planning.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const FIXTURE = join(here, '..', '..', 'fixtures', 'sample-authorities.json');

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
  outDir = await mkdtemp(join(tmpdir(), 'prerender-'));
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

describe('runPrerender — fixture mode', () => {
  it('publishes only qualifying, gated authorities and writes their index.html', async () => {
    const result = await runPrerender({
      outDir,
      fixturePath: FIXTURE,
      logger: silentLogger,
    });

    expect(result.skipped).toBe(false);
    expect(result.published).toEqual(['basingstoke-and-deane']);

    const html = await readFile(
      join(outDir, 'planning', 'basingstoke-and-deane', 'index.html'),
      'utf-8',
    );
    expect(html).toContain(
      '<h1>Planning applications in Basingstoke and Deane</h1>',
    );
    expect(html).toContain('PlanIt');
    expect(html).toContain('Get push alerts for Basingstoke and Deane');
  });

  it('excludes a non-qualifying areaType (English County) and a below-gate authority', async () => {
    const result = await runPrerender({
      outDir,
      fixturePath: FIXTURE,
      logger: silentLogger,
    });

    expect(await exists(join(outDir, 'planning', 'west-sussex'))).toBe(false);
    expect(await exists(join(outDir, 'planning', 'sparse-parish'))).toBe(false);
    expect(result.excluded.map((e) => e.name).sort()).toEqual([
      'Sparse Parish',
      'West Sussex',
    ]);
  });

  it('writes a sitemap.xml listing the published pages', async () => {
    await runPrerender({ outDir, fixturePath: FIXTURE, logger: silentLogger });
    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/basingstoke-and-deane</loc>',
    );
    expect(sitemap).not.toContain('west-sussex');
  });
});

describe('runPrerender — no key, no fixture', () => {
  it('skips gracefully and writes nothing', async () => {
    const result = await runPrerender({
      outDir,
      apiBase: undefined,
      buildKey: undefined,
      fixturePath: undefined,
      logger: silentLogger,
    });

    expect(result.skipped).toBe(true);
    expect(result.published).toEqual([]);
    expect(await exists(join(outDir, 'planning'))).toBe(false);
    expect(await exists(join(outDir, 'sitemap.xml'))).toBe(false);
  });
});

describe('runPrerender — live mode', () => {
  function authoritiesBody(items) {
    return { authorities: items, total: items.length };
  }

  it('fetches the gated endpoint with X-Build-Key and publishes gated authorities only', async () => {
    const stub = new StubFetch((url) => {
      if (url.endsWith('/v1/authorities')) {
        return {
          ok: true,
          status: 200,
          body: authoritiesBody([
            { id: 1, name: 'Adur', areaType: 'English District' },
            { id: 2, name: 'West Sussex', areaType: 'English County' },
          ]),
        };
      }
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
                link: 'https://planit.org.uk/planapplic/A1',
                url: null,
              },
            ],
            total: 12,
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
      logger: silentLogger,
    });

    expect(result.published).toEqual(['adur']);

    // West Sussex (English County) must never trigger an applications fetch.
    const appsCalls = stub.calls.filter((c) =>
      c.url.includes('/applications'),
    );
    expect(appsCalls).toHaveLength(1);
    expect(appsCalls[0].url).toContain('/v1/authorities/1/applications');
    expect(appsCalls[0].url).toContain('limit=30');
    expect(appsCalls[0].init.headers['X-Build-Key']).toBe('test-key');

    const html = await readFile(
      join(outDir, 'planning', 'adur', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<h1>Planning applications in Adur</h1>');
  });

  it('excludes a qualifying authority that fails the coverage gate', async () => {
    const stub = new StubFetch((url) => {
      if (url.endsWith('/v1/authorities')) {
        return {
          ok: true,
          status: 200,
          body: authoritiesBody([
            { id: 1, name: 'Adur', areaType: 'English District' },
          ]),
        };
      }
      return {
        ok: true,
        status: 200,
        body: {
          authorityId: 1,
          areaName: 'Adur',
          applications: [],
          total: 5,
          totalCapped: false,
        },
      };
    });

    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      logger: silentLogger,
    });

    expect(result.published).toEqual([]);
    expect(await exists(join(outDir, 'planning', 'adur'))).toBe(false);
  });

  it('treats zero qualifying authorities as a valid (non-error) build', async () => {
    const stub = new StubFetch(() => ({
      ok: true,
      status: 200,
      body: authoritiesBody([
        { id: 9, name: 'Surrey', areaType: 'English County' },
      ]),
    }));

    const result = await runPrerender({
      outDir,
      apiBase: 'https://api-dev.towncrierapp.uk',
      buildKey: 'test-key',
      fetchImpl: stub.fetch,
      logger: silentLogger,
    });

    expect(result.skipped).toBe(false);
    expect(result.published).toEqual([]);
  });

  it('fails loud when the authorities endpoint returns a non-OK status', async () => {
    const stub = new StubFetch(() => ({ ok: false, status: 500, body: {} }));
    await expect(
      runPrerender({
        outDir,
        apiBase: 'https://api-dev.towncrierapp.uk',
        buildKey: 'test-key',
        fetchImpl: stub.fetch,
        logger: silentLogger,
      }),
    ).rejects.toThrow();
  });

  it('fails loud when the applications endpoint returns an unexpected shape', async () => {
    const stub = new StubFetch((url) => {
      if (url.endsWith('/v1/authorities')) {
        return {
          ok: true,
          status: 200,
          body: authoritiesBody([
            { id: 1, name: 'Adur', areaType: 'English District' },
          ]),
        };
      }
      return { ok: true, status: 200, body: { not: 'what we expect' } };
    });

    await expect(
      runPrerender({
        outDir,
        apiBase: 'https://api-dev.towncrierapp.uk',
        buildKey: 'test-key',
        fetchImpl: stub.fetch,
        logger: silentLogger,
      }),
    ).rejects.toThrow();
  });

  it('fails loud when a build key is set but no API base is configured', async () => {
    await expect(
      runPrerender({
        outDir,
        apiBase: undefined,
        buildKey: 'test-key',
        logger: silentLogger,
      }),
    ).rejects.toThrow();
  });
});
