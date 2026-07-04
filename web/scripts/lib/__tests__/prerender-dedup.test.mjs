import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtemp, rm, readFile, writeFile, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { runPrerender } from '../../prerender-planning.mjs';

const silentLogger = { log() {}, warn() {}, error() {} };

let outDir;
let baseConfigPath;

const BASE_CONFIG = {
  navigationFallback: { rewrite: '/index.html', exclude: ['*.{css,js}'] },
  globalHeaders: { 'X-Frame-Options': 'DENY' },
  routes: [{ route: '/assets/*', headers: { 'Cache-Control': 'public' } }],
};

beforeEach(async () => {
  outDir = await mkdtemp(join(tmpdir(), 'prerender-dedup-'));
  baseConfigPath = join(outDir, 'base-staticwebapp.config.json');
  await writeFile(baseConfigPath, JSON.stringify(BASE_CONFIG), 'utf-8');
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

/**
 * Write authority + town fixtures and run the prerender in fixture mode with an
 * injected base SWA config path. Returns the run result.
 */
async function runWith({ authorities, towns, authorityList }) {
  const authorityFixture = join(outDir, 'authorities.json');
  const townFixture = join(outDir, 'towns.json');
  await writeFile(authorityFixture, JSON.stringify(authorities), 'utf-8');
  await writeFile(townFixture, JSON.stringify(towns), 'utf-8');
  return runPrerender({
    outDir,
    fixturePath: authorityFixture,
    townFixturePath: townFixture,
    baseConfigPath,
    loadAuthorities: async () => authorityList,
    logger: silentLogger,
  });
}

describe('runPrerender — same-name town dedup (tc-77ll)', () => {
  const wrexhamAndCornwall = {
    authorities: [
      {
        id: 10,
        name: 'Wrexham',
        areaType: 'Welsh Principal Area',
        areaName: 'Wrexham',
        total: 20,
        statusBreakdown: [{ appState: 'Permitted', count: 20 }],
        applications: [app('W1')],
      },
      {
        id: 52,
        name: 'Cornwall',
        areaType: 'English Unitary Authority',
        areaName: 'Cornwall',
        total: 30,
        statusBreakdown: [{ appState: 'Permitted', count: 30 }],
        applications: [app('C1')],
      },
    ],
    towns: [
      // same-name town under Wrexham -> suppressed + redirected
      {
        slug: 'wrexham',
        name: 'Wrexham',
        lat: 53.04,
        lng: -2.99,
        authorityId: 10,
        total: 25,
        statusBreakdown: [{ appState: 'Permitted', count: 25 }],
        applications: [app('WT1')],
      },
      // same-name town under Cornwall -> suppressed + redirected
      {
        slug: 'cornwall',
        name: 'Cornwall',
        lat: 50.5,
        lng: -4.6,
        authorityId: 52,
        total: 22,
        statusBreakdown: [{ appState: 'Permitted', count: 22 }],
        applications: [app('CT1')],
      },
      // genuinely different town under Cornwall -> published + linked, no redirect
      {
        slug: 'truro',
        name: 'Truro',
        lat: 50.2632,
        lng: -5.051,
        authorityId: 52,
        total: 18,
        statusBreakdown: [{ appState: 'Permitted', count: 18 }],
        applications: [app('TR1')],
      },
    ],
    authorityList: [
      { id: 10, name: 'Wrexham' },
      { id: 52, name: 'Cornwall' },
    ],
  };

  it('suppresses a same-name town: no page, not published, not in the sitemap', async () => {
    const result = await runWith(wrexhamAndCornwall);

    expect(result.publishedTowns).toEqual(['cornwall/truro']);
    expect(result.publishedTowns).not.toContain('wrexham/wrexham');
    expect(result.publishedTowns).not.toContain('cornwall/cornwall');

    expect(
      await exists(join(outDir, 'planning', 'wrexham', 'wrexham', 'index.html')),
    ).toBe(false);
    expect(
      await exists(join(outDir, 'planning', 'cornwall', 'cornwall', 'index.html')),
    ).toBe(false);
    // the genuinely different town still publishes
    expect(
      await exists(join(outDir, 'planning', 'cornwall', 'truro', 'index.html')),
    ).toBe(true);

    const sitemap = await readFile(join(outDir, 'sitemap.xml'), 'utf-8');
    expect(sitemap).toContain(
      '<loc>https://towncrierapp.uk/planning/cornwall/truro</loc>',
    );
    expect(sitemap).not.toContain('/planning/wrexham/wrexham');
    expect(sitemap).not.toContain('/planning/cornwall/cornwall');
  });

  it('records a 301 redirect for each suppressed town, none for a published town', async () => {
    const result = await runWith(wrexhamAndCornwall);

    expect(result.redirects.sort()).toEqual([
      'cornwall/cornwall',
      'wrexham/wrexham',
    ]);
    expect(result.redirects).not.toContain('cornwall/truro');
  });

  it('merges the 301 routes into the base SWA config written to the outDir', async () => {
    await runWith(wrexhamAndCornwall);

    const merged = JSON.parse(
      await readFile(join(outDir, 'staticwebapp.config.json'), 'utf-8'),
    );
    expect(merged.routes).toContainEqual({
      route: '/planning/wrexham/wrexham',
      redirect: '/planning/wrexham',
      statusCode: 301,
    });
    expect(merged.routes).toContainEqual({
      route: '/planning/cornwall/cornwall',
      redirect: '/planning/cornwall',
      statusCode: 301,
    });
    // hand-written base route survives the merge.
    expect(merged.routes).toContainEqual({
      route: '/assets/*',
      headers: { 'Cache-Control': 'public' },
    });
    // and the rest of the base config is untouched.
    expect(merged.navigationFallback).toEqual(BASE_CONFIG.navigationFallback);
    expect(merged.globalHeaders).toEqual(BASE_CONFIG.globalHeaders);
  });

  it('drops the suppressed town from the authority page town-links (no self-link)', async () => {
    await runWith(wrexhamAndCornwall);

    // Cornwall links down to Truro but NEVER to the suppressed same-name town.
    const cornwall = await readFile(
      join(outDir, 'planning', 'cornwall', 'index.html'),
      'utf-8',
    );
    expect(cornwall).toContain('<a href="/planning/cornwall/truro">Truro</a>');
    expect(cornwall).not.toContain('/planning/cornwall/cornwall');

    // Wrexham has 0 qualifying towns -> the town-links section is omitted.
    const wrexham = await readFile(
      join(outDir, 'planning', 'wrexham', 'index.html'),
      'utf-8',
    );
    expect(wrexham).not.toContain('<section class="townLinks">');
    expect(wrexham).not.toContain('/planning/wrexham/wrexham');
  });

  it('classifies a suppressed town as excluded with reason "same-name"', async () => {
    const result = await runWith(wrexhamAndCornwall);
    expect(result.excludedTowns).toContainEqual({
      name: 'Wrexham',
      reason: 'same-name',
    });
    expect(result.excludedTowns).toContainEqual({
      name: 'Cornwall',
      reason: 'same-name',
    });
  });

  it('suppresses a forward-looking "City of" authority/town collision', async () => {
    const result = await runWith({
      authorities: [
        {
          id: 7,
          name: 'Bristol, City of',
          areaType: 'English Unitary Authority',
          areaName: 'Bristol, City of',
          total: 40,
          statusBreakdown: [{ appState: 'Permitted', count: 40 }],
          applications: [app('B1')],
        },
      ],
      towns: [
        {
          slug: 'bristol',
          name: 'Bristol',
          lat: 51.4545,
          lng: -2.5879,
          authorityId: 7,
          total: 30,
          statusBreakdown: [{ appState: 'Permitted', count: 30 }],
          applications: [app('BT1')],
        },
      ],
      authorityList: [{ id: 7, name: 'Bristol, City of' }],
    });

    expect(result.publishedTowns).toEqual([]);
    expect(result.redirects).toEqual(['bristol-city-of/bristol']);
    const merged = JSON.parse(
      await readFile(join(outDir, 'staticwebapp.config.json'), 'utf-8'),
    );
    expect(merged.routes).toContainEqual({
      route: '/planning/bristol-city-of/bristol',
      redirect: '/planning/bristol-city-of',
      statusCode: 301,
    });
  });

  it('does not redirect a same-name town that never cleared the coverage gate', async () => {
    const result = await runWith({
      authorities: [
        {
          id: 10,
          name: 'Wrexham',
          areaType: 'Welsh Principal Area',
          areaName: 'Wrexham',
          total: 20,
          statusBreakdown: [{ appState: 'Permitted', count: 20 }],
          applications: [app('W1')],
        },
      ],
      towns: [
        {
          slug: 'wrexham',
          name: 'Wrexham',
          lat: 53.04,
          lng: -2.99,
          authorityId: 10,
          total: 3, // below the >=10 coverage gate -> never published, no redirect
          statusBreakdown: [{ appState: 'Permitted', count: 3 }],
          applications: [],
        },
      ],
      authorityList: [{ id: 10, name: 'Wrexham' }],
    });

    // gated out for coverage, not same-name; a never-published URL needs no 301.
    expect(result.redirects).toEqual([]);
    expect(result.excludedTowns).toContainEqual({
      name: 'Wrexham',
      reason: 'coverage',
    });
  });
});

describe('real-world case: Croydon shows zero town links (tc-r4n9.4 investigation)', () => {
  it('documents why: the gazetteer carries exactly one Croydon-authority row, named "Croydon" itself, so the pre-existing same-name dedup (tc-77ll) suppresses it -- not an ordering regression or a stale snapshot', async () => {
    // Mirrors the real committed values in web/src/data/towns.json for
    // authorityId 301 (Croydon): a SINGLE row, name "Croydon". ONS's Census 2021
    // Built-Up-Areas methodology treats each Greater London borough as one
    // borough-shaped BUA rather than distinct settlements (see the "London
    // population" note in generate-towns.mjs) -- there is no separate
    // "Purley" / "Coulsdon" / "South Norwood" row for this dedup to spare.
    const result = await runWith({
      authorities: [
        {
          id: 301,
          name: 'Croydon',
          areaType: 'London Borough',
          areaName: 'Croydon',
          total: 40,
          statusBreakdown: [{ appState: 'Permitted', count: 40 }],
          applications: [app('CR1')],
        },
      ],
      towns: [
        {
          slug: 'croydon',
          name: 'Croydon',
          lat: 51.3507,
          lng: -0.083,
          authorityId: 301,
          total: 30, // clears the >=10 coverage gate; suppressed on same-name below
          statusBreakdown: [{ appState: 'Permitted', count: 30 }],
          applications: [app('CRT1')],
        },
      ],
      authorityList: [{ id: 301, name: 'Croydon' }],
    });

    expect(result.publishedTowns).toEqual([]);
    expect(result.excludedTowns).toContainEqual({
      name: 'Croydon',
      reason: 'same-name',
    });

    const croydonPage = await readFile(
      join(outDir, 'planning', 'croydon', 'index.html'),
      'utf-8',
    );
    expect(croydonPage).not.toContain('<section class="townLinks">');
    // The breadcrumb (tc-r4n9.4) still renders regardless of the town-links gap.
    expect(croydonPage).toContain('class="breadcrumb"');
  });
});
