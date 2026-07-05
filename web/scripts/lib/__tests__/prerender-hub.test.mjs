import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, readFile, writeFile, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { runPrerender, runRender } from '../../prerender-planning.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const AUTHORITIES_FIXTURE = join(here, '..', '..', 'fixtures', 'sample-authorities.json');

/**
 * Coverage for the `/planning/` authority hub page (tc-geq7h.1, GH #821 Phase
 * 1): rendered by the SAME `renderEntries` pipeline every mode shares, from
 * data ALREADY present in the snapshot/fixture — zero new API calls, zero new
 * fetch surface. Exercised primarily through `runRender` (the offline
 * `--render` mode this bead targets), plus one fixture-mode check proving the
 * hub is wired into every entry point, not bolted onto one.
 */

const silentLogger = { log() {}, warn() {}, error() {} };

let outDir;

beforeEach(async () => {
  outDir = await mkdtemp(join(tmpdir(), 'prerender-hub-'));
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

/** @param {string} uid @param {string} lastDifferent */
function app(uid, lastDifferent) {
  return {
    uid,
    name: uid,
    address: `${uid} address`,
    description: 'desc',
    appState: 'Permitted',
    startDate: '2026-01-10',
    lastDifferent,
    link: null,
    url: null,
  };
}

/**
 * A snapshot with two qualifying authorities (Adur, Basingstoke and Deane) —
 * deliberately serialised in REVERSE alphabetical order, so an index.html that
 * lists them A-Z proves the prerender sorts before handing entries to the
 * renderer — and one published town under Adur only.
 *
 * @param {string} path
 */
async function writeReverseOrderSnapshot(path) {
  await writeFile(
    path,
    JSON.stringify({
      version: 1,
      generatedAt: '2026-06-25T00:00:00.000Z',
      minPopulation: 20000,
      limit: 30,
      authorities: [
        { id: 2, name: 'Basingstoke and Deane' },
        { id: 1, name: 'Adur' },
      ],
      authorityPages: [
        {
          id: 2,
          name: 'Basingstoke and Deane',
          areaType: 'English District',
          areaName: 'Basingstoke and Deane',
          total: 42,
          statusBreakdown: [{ appState: 'Permitted', count: 42 }],
          applications: [app('BD1', '2026-06-15T10:00:00+00:00')],
        },
        {
          id: 1,
          name: 'Adur',
          areaType: 'English District',
          areaName: 'Adur',
          total: 12,
          statusBreakdown: [{ appState: 'Permitted', count: 12 }],
          applications: [app('A1', '2026-06-10T10:00:00+00:00')],
        },
      ],
      townPages: [
        {
          slug: 'lancing',
          name: 'Lancing',
          lat: 50.83,
          lng: -0.32,
          authorityId: 1,
          population: 30000,
          total: 14,
          statusBreakdown: [{ appState: 'Permitted', count: 14 }],
          // Freshest lastDifferent of the whole snapshot — the hub's own
          // sitemap lastmod must pick this up as the max across ALL children.
          applications: [app('L1', '2026-06-25T10:00:00+00:00')],
        },
      ],
    }),
    'utf-8',
  );
}

describe('runRender — /planning/ hub page (tc-geq7h.1)', () => {
  it('writes a real dist/planning/index.html, distinct from any authority-slug page', async () => {
    const snapshotPath = join(outDir, 'seo-snapshot.json');
    await writeReverseOrderSnapshot(snapshotPath);

    await runRender({ outDir, snapshotPath, logger: silentLogger });

    expect(await exists(join(outDir, 'planning', 'index.html'))).toBe(true);
    // Distinct from an authority page — no slug directory is literally "index".
    expect(await exists(join(outDir, 'planning', 'index'))).toBe(false);
  });

  it('lists every published authority A-Z regardless of snapshot order, each with its application and town counts', async () => {
    const snapshotPath = join(outDir, 'seo-snapshot.json');
    await writeReverseOrderSnapshot(snapshotPath);

    await runRender({ outDir, snapshotPath, logger: silentLogger });

    const html = await readFile(join(outDir, 'planning', 'index.html'), 'utf-8');
    expect(html).toContain('<h1>Planning applications by council</h1>');
    expect(html).toContain('<a class="hubList__link" href="/planning/adur">Adur</a>');
    expect(html).toContain(
      '<a class="hubList__link" href="/planning/basingstoke-and-deane">Basingstoke and Deane</a>',
    );
    // Adur's one published town (Lancing) is counted; Basingstoke has none.
    expect(html).toContain('12 applications tracked · 1 town');
    expect(html).toContain('42 applications tracked</span>');
    expect(html).not.toContain('42 applications tracked · 0 towns');
    // A-Z order, even though the snapshot serialised Basingstoke first.
    expect(html.indexOf('>Adur<')).toBeLessThan(
      html.indexOf('>Basingstoke and Deane<'),
    );
  });

  it('adds a sitemap entry for the bare /planning root, lastmod = the max across every child page', async () => {
    const snapshotPath = join(outDir, 'seo-snapshot.json');
    await writeReverseOrderSnapshot(snapshotPath);

    await runRender({ outDir, snapshotPath, logger: silentLogger });

    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    expect(sitemap).toContain('<loc>https://towncrierapp.uk/planning</loc>');
    expect(sitemap).not.toContain('<loc>https://towncrierapp.uk/planning/</loc>');
    // Lancing's 25 Jun app is the freshest of the three (15/10/25 Jun) — the
    // hub's lastmod must reflect it, not just the authority pages' own dates.
    expect(sitemap).toContain('<lastmod>2026-06-25</lastmod>');
  });

  it('never issues any network call while rendering the hub from the snapshot', async () => {
    const snapshotPath = join(outDir, 'seo-snapshot.json');
    await writeReverseOrderSnapshot(snapshotPath);

    const calls = [];
    const original = globalThis.fetch;
    globalThis.fetch = (...args) => {
      calls.push(args);
      throw new Error('render mode must not perform any network I/O');
    };
    try {
      await runRender({ outDir, snapshotPath, logger: silentLogger });
    } finally {
      globalThis.fetch = original;
    }
    expect(calls).toHaveLength(0);
    expect(await exists(join(outDir, 'planning', 'index.html'))).toBe(true);
  });
});

describe('runPrerender — fixture mode also emits the hub page', () => {
  it('writes dist/planning/index.html from a fixture, same as --render', async () => {
    await runPrerender({
      outDir,
      fixturePath: AUTHORITIES_FIXTURE,
      logger: silentLogger,
    });

    expect(await exists(join(outDir, 'planning', 'index.html'))).toBe(true);
    const html = await readFile(join(outDir, 'planning', 'index.html'), 'utf-8');
    expect(html).toContain(
      '<a class="hubList__link" href="/planning/basingstoke-and-deane">Basingstoke and Deane</a>',
    );
  });
});
