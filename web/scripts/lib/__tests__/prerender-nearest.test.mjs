import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, readFile, writeFile, readdir, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { runPrerender } from '../../prerender-planning.mjs';

/**
 * Two-pass restructuring coverage (tc-gyw1q, GH #990 slices 3+4): a page's
 * neighbour links require knowing the FULL published set before any page can
 * be written, so these tests exercise the whole `runPrerender` fixture-mode
 * pipeline end to end rather than the individual decide/write helpers.
 */

const silentLogger = { log() {}, warn() {}, error() {} };

let outDir;

beforeEach(async () => {
  outDir = await mkdtemp(join(tmpdir(), 'prerender-nearest-'));
});

afterEach(async () => {
  await rm(outDir, { recursive: true, force: true });
});

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

/**
 * A dense fixture: 9 qualifying authorities, 8 of which have published towns
 * (giving each other 7 other centroid candidates so nearestK(6) has to
 * TRUNCATE, not just return everything), plus one authority (Iardshire) with
 * ZERO published towns of its own — no honest centroid, so it must be
 * excluded both from having its own section AND from being a neighbour
 * CANDIDATE for anyone else. Every town sits at the same longitude with
 * strictly increasing latitude, so distance ordering is exact and easy to
 * reason about by hand.
 */
const authorities = [
  { id: 1, name: 'Aardshire', areaType: 'English District' },
  { id: 2, name: 'Bardshire', areaType: 'English District' },
  { id: 3, name: 'Cardshire', areaType: 'English District' },
  { id: 4, name: 'Dardshire', areaType: 'English District' },
  { id: 5, name: 'Eardshire', areaType: 'English District' },
  { id: 6, name: 'Fardshire', areaType: 'English District' },
  { id: 7, name: 'Gardshire', areaType: 'English District' },
  { id: 8, name: 'Hardshire', areaType: 'English District' },
  { id: 9, name: 'Iardshire', areaType: 'English District' },
];

function authorityFixtureEntries() {
  return authorities.map((a) => ({
    id: a.id,
    name: a.name,
    areaType: a.areaType,
    areaName: a.name,
    total: 15,
    statusBreakdown: [{ appState: 'Permitted', count: 15 }],
    applications: [app(`${a.name}-A1`)],
  }));
}

/** slug -> lat, in strictly increasing order. */
const townFixtureEntries = [
  { slug: 'apton', name: 'Apton', lat: 51.0, lng: -1.0, authorityId: 1 },
  { slug: 'bapton', name: 'Bapton', lat: 51.02, lng: -1.0, authorityId: 1 },
  { slug: 'capton', name: 'Capton', lat: 51.05, lng: -1.0, authorityId: 2 },
  { slug: 'japton', name: 'Japton', lat: 51.06, lng: -1.0, authorityId: 2 },
  { slug: 'dapton', name: 'Dapton', lat: 51.1, lng: -1.0, authorityId: 3 },
  { slug: 'eapton', name: 'Eapton', lat: 51.2, lng: -1.0, authorityId: 4 },
  { slug: 'fapton', name: 'Fapton', lat: 51.4, lng: -1.0, authorityId: 5 },
  { slug: 'gapton', name: 'Gapton', lat: 51.7, lng: -1.0, authorityId: 6 },
  { slug: 'hapton', name: 'Hapton', lat: 52.1, lng: -1.0, authorityId: 7 },
  { slug: 'iapton', name: 'Iapton', lat: 52.12, lng: -1.0, authorityId: 7 },
  { slug: 'kapton', name: 'Kapton', lat: 52.5, lng: -1.0, authorityId: 8 },
  // authorityId 9 (Iardshire) deliberately has NO town at all.
].map((t) => ({
  ...t,
  total: 12,
  statusBreakdown: [{ appState: 'Permitted', count: 12 }],
  applications: [app(`${t.name}-T1`)],
}));

async function writeDenseFixtures(dir) {
  const authorityFixture = join(dir, 'authorities.json');
  const townFixture = join(dir, 'towns.json');
  await writeFile(authorityFixture, JSON.stringify(authorityFixtureEntries()), 'utf-8');
  await writeFile(townFixture, JSON.stringify(townFixtureEntries), 'utf-8');
  return { authorityFixture, townFixture };
}

async function runDense(dir) {
  const { authorityFixture, townFixture } = await writeDenseFixtures(dir);
  return runPrerender({
    outDir: dir,
    fixturePath: authorityFixture,
    townFixturePath: townFixture,
    loadAuthorities: async () => authorities,
    logger: silentLogger,
  });
}

describe('runPrerender — nearby areas (town pages, tc-gyw1q GH #990 slice 3)', () => {
  it('renders exactly 8 nearby-area links, truncating the two farthest candidates', async () => {
    await runDense(outDir);

    const html = await readFile(
      join(outDir, 'planning', 'aardshire', 'apton', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<section class="nearbyAreas">');
    const section = html.match(/<section class="nearbyAreas">[\s\S]*?<\/section>/)[0];
    const liCount = (section.match(/<li>/g) ?? []).length;
    expect(liCount).toBe(8);

    // The two FARTHEST towns from Apton (Iapton, Kapton) are truncated away.
    expect(section).not.toContain('>Iapton');
    expect(section).not.toContain('>Kapton');
    // The nearest same-authority town is still included...
    expect(section).toContain('Bapton, Aardshire');
    // ...alongside a genuinely cross-authority neighbour (the border-town case).
    expect(section).toContain('Capton, Bardshire');
  });

  it('links each nearby town to its own nested /planning/<authority>/<town> path', async () => {
    await runDense(outDir);

    const html = await readFile(
      join(outDir, 'planning', 'aardshire', 'apton', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<a href="/planning/bardshire/capton">Capton, Bardshire</a>');
  });

  it('renders fewer than 8 links when the published town pool is smaller', async () => {
    // Only 3 towns published in total -> each gets at most 2 candidates.
    const authorityFixture = join(outDir, 'authorities-small.json');
    const townFixture = join(outDir, 'towns-small.json');
    await writeFile(
      authorityFixture,
      JSON.stringify([
        {
          id: 1,
          name: 'Aardshire',
          areaType: 'English District',
          areaName: 'Aardshire',
          total: 15,
          statusBreakdown: [{ appState: 'Permitted', count: 15 }],
          applications: [app('A1')],
        },
      ]),
      'utf-8',
    );
    await writeFile(
      townFixture,
      JSON.stringify(
        [
          { slug: 'apton', name: 'Apton', lat: 51.0, lng: -1.0, authorityId: 1 },
          { slug: 'bapton', name: 'Bapton', lat: 51.02, lng: -1.0, authorityId: 1 },
          { slug: 'capton', name: 'Capton', lat: 51.05, lng: -1.0, authorityId: 1 },
        ].map((t) => ({
          ...t,
          total: 12,
          statusBreakdown: [{ appState: 'Permitted', count: 12 }],
          applications: [app(`${t.name}-T1`)],
        })),
      ),
      'utf-8',
    );

    await runPrerender({
      outDir,
      fixturePath: authorityFixture,
      townFixturePath: townFixture,
      loadAuthorities: async () => [{ id: 1, name: 'Aardshire' }],
      logger: silentLogger,
    });

    const html = await readFile(
      join(outDir, 'planning', 'aardshire', 'apton', 'index.html'),
      'utf-8',
    );
    const section = html.match(/<section class="nearbyAreas">[\s\S]*?<\/section>/)[0];
    const liCount = (section.match(/<li>/g) ?? []).length;
    expect(liCount).toBe(2);
  });
});

describe('runPrerender — neighbouring councils (authority pages, tc-gyw1q GH #990 slice 4)', () => {
  it('renders exactly 6 neighbouring-council links, truncating the farthest candidate', async () => {
    await runDense(outDir);

    const html = await readFile(
      join(outDir, 'planning', 'aardshire', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<section class="neighbouringCouncils">');
    const section = html.match(
      /<section class="neighbouringCouncils">[\s\S]*?<\/section>/,
    )[0];
    const liCount = (section.match(/<li>/g) ?? []).length;
    expect(liCount).toBe(6);

    // Aardshire's centroid is 51.01; Hardshire (52.50) is the FARTHEST of the
    // 7 other authorities with a centroid, so it is truncated away...
    expect(section).not.toContain('>Hardshire<');
    // ...and Iardshire (zero published towns, no centroid) was never even a
    // CANDIDATE, let alone a neighbour.
    expect(section).not.toContain('Iardshire');
    // The 6 nearest all appear.
    for (const name of [
      'Bardshire',
      'Cardshire',
      'Dardshire',
      'Eardshire',
      'Fardshire',
      'Gardshire',
    ]) {
      expect(section).toContain(`>${name}<`);
    }
  });

  it('links each neighbour to its own /planning/<slug> authority page', async () => {
    await runDense(outDir);

    const html = await readFile(
      join(outDir, 'planning', 'aardshire', 'index.html'),
      'utf-8',
    );
    expect(html).toContain('<a href="/planning/bardshire">Bardshire</a>');
  });

  it('renders no neighbours section for an authority with zero published towns, and does not error', async () => {
    const result = await runDense(outDir);
    expect(result.published).toContain('iardshire');

    const html = await readFile(
      join(outDir, 'planning', 'iardshire', 'index.html'),
      'utf-8',
    );
    expect(html).not.toContain('<section class="neighbouringCouncils">');
  });
});

describe('runPrerender — no generated /planning/* link ever points at an unpublished page (GH #990 acceptance)', () => {
  it('every href="/planning/..." across every rendered page resolves to something actually published', async () => {
    const result = await runDense(outDir);

    const publishedPaths = new Set([
      '/planning',
      '/planning/towns',
      ...result.published.map((slug) => `/planning/${slug}`),
      ...result.publishedTowns.map((path) => `/planning/${path}`),
    ]);

    const files = await readAllFiles(join(outDir, 'planning'));
    const pathPattern =
      /(?:https:\/\/towncrierapp\.uk)?(\/planning(?:\/[a-z0-9][a-z0-9-]*){0,2})(?=["\s])/g;

    let checked = 0;
    for (const [, content] of files) {
      for (const match of content.matchAll(pathPattern)) {
        const path = match[1];
        expect(publishedPaths.has(path), `unpublished link: ${path}`).toBe(true);
        checked += 1;
      }
    }
    // Sanity check the scan actually exercised a meaningful number of links —
    // an empty scan would make every assertion above vacuously true.
    expect(checked).toBeGreaterThan(20);
  });
});

describe('runPrerender — two consecutive render passes are byte-identical (GH #990 acceptance)', () => {
  it('produces identical HTML for every page across two independent renders of the same fixtures', async () => {
    const outDirA = await mkdtemp(join(tmpdir(), 'prerender-nearest-a-'));
    const outDirB = await mkdtemp(join(tmpdir(), 'prerender-nearest-b-'));
    try {
      await runDense(outDirA);
      await runDense(outDirB);

      const filesA = await readAllFiles(join(outDirA, 'planning'));
      const filesB = await readAllFiles(join(outDirB, 'planning'));

      expect([...filesA.keys()].sort()).toEqual([...filesB.keys()].sort());
      for (const [relPath, contentA] of filesA) {
        expect(contentA, relPath).toBe(filesB.get(relPath));
      }
    } finally {
      await rm(outDirA, { recursive: true, force: true });
      await rm(outDirB, { recursive: true, force: true });
    }
  });
});

/**
 * Recursively read every FILE under `dir` into a `relativePath -> content`
 * map (relative to `dir`), for structural/content comparison across renders.
 *
 * @param {string} dir
 * @returns {Promise<Map<string, string>>}
 */
async function readAllFiles(dir) {
  const relPaths = await readdir(dir, { recursive: true });
  const files = new Map();
  for (const rel of relPaths) {
    const full = join(dir, rel);
    const st = await stat(full);
    if (st.isFile()) {
      files.set(rel, await readFile(full, 'utf-8'));
    }
  }
  return files;
}
