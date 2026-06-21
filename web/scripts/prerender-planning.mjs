/**
 * Static prerender for the programmatic authority-level SEO pages
 * (`/planning/<slug>`). Runs AFTER `vite build`, emitting fully static,
 * hydration-free HTML into `web/dist/planning/<slug>/index.html` plus a
 * `web/dist/sitemap.xml`.
 *
 * Three run modes, selected by environment:
 *   - SITE_BUILD_KEY set        -> live: read the committed authority list from
 *                                  the repo, then fetch each authority's recent
 *                                  applications from our build-key-gated API.
 *                                  Fail LOUD on any transport/shape error
 *                                  (never empty pages).
 *   - PRERENDER_FIXTURE=<path>  -> fixture: render from a committed JSON file,
 *                                  no network (used by the tracer + tests).
 *   - neither set               -> skip: leave the SPA build untouched so
 *                                  `npm run build` still succeeds without a key.
 *
 * The authority list itself is static, public, committed data (read from disk,
 * not over HTTP — the live `/v1/authorities` endpoint needs an Auth0 token). The
 * build key is server-side only and is NEVER written into any page or the client
 * bundle. We read ONLY our own API — never PlanIt (ADR 0006).
 */

import { mkdir, writeFile, readFile } from 'node:fs/promises';
import { join, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { slugify } from './lib/slug.mjs';
import { isQualifyingAreaType } from './lib/area-types.mjs';
import { meetsCoverageGate } from './lib/coverage-gate.mjs';
import { renderPlanningPage } from './lib/render-page.mjs';
import { renderTownPage } from './lib/render-town-page.mjs';
import { resolveAuthority, townPagePath } from './lib/town-path.mjs';
import { renderSitemap } from './lib/render-sitemap.mjs';

const DEFAULT_LIMIT = 30;

/** Directory containing this script (`web/scripts`), independent of cwd. */
const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));

/**
 * The authority list is static, public, committed data — read it from the repo
 * rather than over HTTP. The live `GET /v1/authorities` endpoint requires an
 * Auth0 bearer token (401 anonymously), and only the per-authority applications
 * endpoint is build-key-anonymous. The whole monorepo is checked out in CI, so
 * this file is always present two directories up from `web/scripts`.
 * @type {string}
 */
export const AUTHORITIES_FILE = join(
  SCRIPT_DIR,
  '..',
  '..',
  'api-go',
  'internal',
  'authorities',
  'resources',
  'authorities.json',
);

/**
 * The town gazetteer is a slim, committed JSON file in the web app's source
 * tree (`web/src/data/towns.json`). It is regenerated occasionally by
 * `scripts/generate-towns.mjs` from OS Open Names — never downloaded at build
 * time. Each row is `{ slug, name, lat, lng, authorityId }`.
 * @type {string}
 */
export const TOWNS_FILE = join(SCRIPT_DIR, '..', 'src', 'data', 'towns.json');

/**
 * @typedef {Object} PrerenderResult
 * @property {boolean} skipped
 * @property {string} [reason]
 * @property {string[]} published                       authority slugs written
 * @property {Array<{ name: string, reason: string }>} excluded
 * @property {string[]} publishedTowns                  town paths written (<authority>/<town>)
 * @property {Array<{ name: string, reason: string }>} excludedTowns
 */

/**
 * @param {string} base
 * @returns {string} base URL with any trailing slash removed
 */
function trimTrailingSlash(base) {
  return base.replace(/\/+$/, '');
}

/**
 * Load and validate the full authority list from the committed JSON file.
 * Throws loudly if the file is missing, is not valid JSON, is not a non-empty
 * array, or has a malformed row — never a silent empty list.
 *
 * @param {string} filePath
 * @param {(path: string, encoding: string) => Promise<string>} readFileImpl
 * @returns {Promise<Array<{ id: number, name: string, areaType: string }>>}
 */
export async function loadAuthoritiesFromFile(filePath, readFileImpl) {
  let raw;
  try {
    raw = await readFileImpl(filePath, 'utf-8');
  } catch (err) {
    throw new Error(
      `authority list not found at ${filePath}: ` +
        `${err instanceof Error ? err.message : String(err)}`,
    );
  }

  let list;
  try {
    list = JSON.parse(raw);
  } catch (err) {
    throw new Error(
      `authority list at ${filePath} is not valid JSON: ` +
        `${err instanceof Error ? err.message : String(err)}`,
    );
  }

  if (!Array.isArray(list) || list.length === 0) {
    throw new Error(
      `authority list at ${filePath} must be a non-empty JSON array`,
    );
  }
  for (const a of list) {
    if (
      typeof a?.id !== 'number' ||
      typeof a?.name !== 'string' ||
      typeof a?.areaType !== 'string'
    ) {
      throw new Error(
        `authority list at ${filePath} has a malformed authority row`,
      );
    }
  }
  return list;
}

/**
 * @typedef {Object} Town
 * @property {string} slug          lowercase-hyphenated, e.g. "truro"
 * @property {string} name          display name, e.g. "Truro"
 * @property {number} lat           WGS84 latitude (centroid)
 * @property {number} lng           WGS84 longitude (centroid)
 * @property {number} authorityId   parent authority id (resolves to its slug)
 */

/**
 * Load and validate the slim committed town gazetteer. An empty array is valid
 * (sparse data -> zero town pages is a legitimate build), but the file must be
 * present, parse as JSON, be an array, and every row must be well-formed —
 * never a silent malformed gazetteer.
 *
 * @param {string} filePath
 * @param {(path: string, encoding: string) => Promise<string>} readFileImpl
 * @returns {Promise<Town[]>}
 */
export async function loadTownsFromFile(filePath, readFileImpl) {
  let raw;
  try {
    raw = await readFileImpl(filePath, 'utf-8');
  } catch (err) {
    throw new Error(
      `town gazetteer not found at ${filePath}: ` +
        `${err instanceof Error ? err.message : String(err)}`,
    );
  }

  let list;
  try {
    list = JSON.parse(raw);
  } catch (err) {
    throw new Error(
      `town gazetteer at ${filePath} is not valid JSON: ` +
        `${err instanceof Error ? err.message : String(err)}`,
    );
  }

  if (!Array.isArray(list)) {
    throw new Error(`town gazetteer at ${filePath} must be a JSON array`);
  }
  for (const t of list) {
    if (
      typeof t?.slug !== 'string' ||
      t.slug.length === 0 ||
      typeof t?.name !== 'string' ||
      t.name.length === 0 ||
      !Number.isFinite(t?.lat) ||
      !Number.isFinite(t?.lng) ||
      !Number.isFinite(t?.authorityId)
    ) {
      throw new Error(`town gazetteer at ${filePath} has a malformed town row`);
    }
  }
  return list;
}

/**
 * Fetch the bounded recent-applications projection for one authority via the
 * build-key-gated endpoint. Throws on any non-OK status or unexpected shape.
 *
 * @param {string} apiBase
 * @param {number} authorityId
 * @param {string} buildKey
 * @param {number} limit
 * @param {typeof globalThis.fetch} fetchImpl
 * @returns {Promise<{ areaName: string, applications: object[], total: number, totalCapped: boolean }>}
 */
async function fetchRecentApplications(
  apiBase,
  authorityId,
  buildKey,
  limit,
  fetchImpl,
) {
  const url = `${apiBase}/v1/authorities/${authorityId}/applications?limit=${limit}`;
  const res = await fetchImpl(url, { headers: { 'X-Build-Key': buildKey } });
  if (!res.ok) {
    throw new Error(
      `GET /v1/authorities/${authorityId}/applications failed: HTTP ${res.status}`,
    );
  }
  const body = await res.json();
  if (
    !body ||
    !Array.isArray(body.applications) ||
    typeof body.total !== 'number' ||
    typeof body.totalCapped !== 'boolean'
  ) {
    throw new Error(
      `GET /v1/authorities/${authorityId}/applications returned an unexpected shape`,
    );
  }
  return {
    areaName: typeof body.areaName === 'string' ? body.areaName : '',
    applications: body.applications,
    total: body.total,
    totalCapped: body.totalCapped,
  };
}

/**
 * Write one authority's page to <outDir>/planning/<slug>/index.html.
 *
 * @param {string} outDir
 * @param {import('./lib/render-page.mjs').PlanningPageData} data
 * @returns {Promise<void>}
 */
async function writePage(outDir, data) {
  const dir = join(outDir, 'planning', data.slug);
  await mkdir(dir, { recursive: true });
  await writeFile(join(dir, 'index.html'), renderPlanningPage(data), 'utf-8');
}

/**
 * Render and write a page if the authority qualifies and clears the gate.
 * Mutates `published`/`excluded`/`seenSlugs` and returns nothing.
 *
 * @param {Object} args
 * @param {string} args.outDir
 * @param {number} args.authorityId
 * @param {string} args.name
 * @param {string} args.areaType
 * @param {string} args.areaName
 * @param {number} args.total
 * @param {boolean} args.totalCapped
 * @param {object[]} args.applications
 * @param {number} args.limit
 * @param {string[]} args.published
 * @param {Array<{ name: string, reason: string }>} args.excluded
 * @param {Set<string>} args.seenSlugs
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<void>}
 */
async function considerAuthority(args) {
  const {
    outDir,
    authorityId,
    name,
    total,
    totalCapped,
    applications,
    areaName,
    limit,
    published,
    excluded,
    seenSlugs,
    logger,
  } = args;

  if (!meetsCoverageGate(total)) {
    excluded.push({ name, reason: 'coverage' });
    return;
  }

  const slug = slugify(name);
  if (seenSlugs.has(slug)) {
    logger.warn(`[prerender] duplicate slug "${slug}" for "${name}" — skipped`);
    excluded.push({ name, reason: 'duplicate-slug' });
    return;
  }
  seenSlugs.add(slug);

  await writePage(outDir, {
    slug,
    areaName: areaName || name,
    authorityId,
    total,
    totalCapped,
    applications: applications.slice(0, limit),
  });
  published.push(slug);
}

/**
 * Fetch the bounded recent-applications-near-a-point projection for one town via
 * the build-key-gated geo endpoint. Scopes the spatial query to the town's
 * authority partition (authorityId) and centroid (lat/lng); the server defaults
 * and clamps the radius. Throws on any non-OK status or unexpected shape.
 *
 * @param {string} apiBase
 * @param {Town} town
 * @param {string} buildKey
 * @param {number} limit
 * @param {typeof globalThis.fetch} fetchImpl
 * @returns {Promise<{ applications: object[], total: number, totalCapped: boolean }>}
 */
async function fetchRecentNearby(apiBase, town, buildKey, limit, fetchImpl) {
  const url =
    `${apiBase}/v1/applications/near?authorityId=${town.authorityId}` +
    `&lat=${town.lat}&lng=${town.lng}&limit=${limit}`;
  const res = await fetchImpl(url, { headers: { 'X-Build-Key': buildKey } });
  if (!res.ok) {
    throw new Error(
      `GET /v1/applications/near (authority ${town.authorityId}, ${town.name}) ` +
        `failed: HTTP ${res.status}`,
    );
  }
  const body = await res.json();
  if (
    !body ||
    !Array.isArray(body.applications) ||
    typeof body.total !== 'number' ||
    typeof body.totalCapped !== 'boolean'
  ) {
    throw new Error(
      `GET /v1/applications/near (authority ${town.authorityId}, ${town.name}) ` +
        `returned an unexpected shape`,
    );
  }
  return {
    applications: body.applications,
    total: body.total,
    totalCapped: body.totalCapped,
  };
}

/**
 * Write one town's page to <outDir>/planning/<authority-slug>/<town-slug>/index.html.
 *
 * @param {string} outDir
 * @param {import('./lib/render-town-page.mjs').TownPageData} data
 * @returns {Promise<void>}
 */
async function writeTownPage(outDir, data) {
  const dir = join(outDir, 'planning', data.authoritySlug, data.townSlug);
  await mkdir(dir, { recursive: true });
  await writeFile(join(dir, 'index.html'), renderTownPage(data), 'utf-8');
}

/**
 * Render and write a town page if it clears the coverage gate, after resolving
 * its parent authority slug. Mutates `publishedTowns`/`excludedTowns`/`seenPaths`.
 *
 * @param {Object} args
 * @param {string} args.outDir
 * @param {Town} args.town
 * @param {ReadonlyArray<{ id: number, name: string }>} args.authorities
 * @param {number} args.total
 * @param {boolean} args.totalCapped
 * @param {object[]} args.applications
 * @param {number} args.limit
 * @param {string[]} args.publishedTowns
 * @param {Array<{ name: string, reason: string }>} args.excludedTowns
 * @param {Set<string>} args.seenPaths
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<void>}
 */
async function considerTown(args) {
  const {
    outDir,
    town,
    authorities,
    total,
    totalCapped,
    applications,
    limit,
    publishedTowns,
    excludedTowns,
    seenPaths,
    logger,
  } = args;

  if (!meetsCoverageGate(total)) {
    excludedTowns.push({ name: town.name, reason: 'coverage' });
    return;
  }

  const { name: authorityName, slug: authoritySlug } = resolveAuthority(
    town.authorityId,
    authorities,
  );
  const path = townPagePath(authoritySlug, town.slug);
  if (seenPaths.has(path)) {
    logger.warn(`[prerender] duplicate town path "${path}" — skipped`);
    excludedTowns.push({ name: town.name, reason: 'duplicate-path' });
    return;
  }
  seenPaths.add(path);

  await writeTownPage(outDir, {
    townName: town.name,
    townSlug: town.slug,
    authorityName,
    authoritySlug,
    authorityId: town.authorityId,
    total,
    totalCapped,
    applications: applications.slice(0, limit),
  });
  publishedTowns.push(path);
}

/**
 * Render every town that clears the coverage gate. `getGeo` resolves a town to
 * its bounded recent-nearby projection (a live geo fetch, or inline fixture
 * data). The parent authority slug is resolved from `authorities`.
 *
 * @param {Object} args
 * @param {string} args.outDir
 * @param {Town[]} args.towns
 * @param {ReadonlyArray<{ id: number, name: string }>} args.authorities
 * @param {(town: Town) => Promise<{ applications: object[], total: number, totalCapped: boolean }>} args.getGeo
 * @param {number} args.limit
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<{ publishedTowns: string[], excludedTowns: Array<{ name: string, reason: string }> }>}
 */
async function renderTownPages(args) {
  const { outDir, towns, authorities, getGeo, limit, logger } = args;

  /** @type {string[]} */
  const publishedTowns = [];
  /** @type {Array<{ name: string, reason: string }>} */
  const excludedTowns = [];
  const seenPaths = new Set();

  for (const town of towns) {
    const geo = await getGeo(town);
    await considerTown({
      outDir,
      town,
      authorities,
      total: geo.total,
      totalCapped: geo.totalCapped,
      applications: Array.isArray(geo.applications) ? geo.applications : [],
      limit,
      publishedTowns,
      excludedTowns,
      seenPaths,
      logger,
    });
  }

  return { publishedTowns, excludedTowns };
}

/**
 * @param {Object} args
 * @param {string} args.outDir
 * @param {string} args.fixturePath               authority fixture (optional)
 * @param {string} [args.townFixturePath]         town fixture (optional)
 * @param {number} args.limit
 * @param {() => Promise<Array<{ id: number, name: string, areaType: string }>>} args.loadAuthorities
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<PrerenderResult>}
 */
async function runFixtureMode(args) {
  const { outDir, fixturePath, townFixturePath, limit, loadAuthorities, logger } =
    args;

  /** @type {string[]} */
  const published = [];
  /** @type {Array<{ name: string, reason: string }>} */
  const excluded = [];
  const seenSlugs = new Set();

  if (fixturePath) {
    const raw = await readFile(fixturePath, 'utf-8');
    const entries = JSON.parse(raw);
    if (!Array.isArray(entries)) {
      throw new Error(`fixture ${fixturePath} must be a JSON array`);
    }
    for (const entry of entries) {
      if (!isQualifyingAreaType(entry.areaType)) {
        excluded.push({ name: entry.name, reason: 'areaType' });
        continue;
      }
      await considerAuthority({
        outDir,
        authorityId: entry.id,
        name: entry.name,
        areaType: entry.areaType,
        areaName: entry.areaName ?? entry.name,
        total: entry.total,
        totalCapped: Boolean(entry.totalCapped),
        applications: Array.isArray(entry.applications)
          ? entry.applications
          : [],
        limit,
        published,
        excluded,
        seenSlugs,
        logger,
      });
    }
  }

  /** @type {string[]} */
  let publishedTowns = [];
  /** @type {Array<{ name: string, reason: string }>} */
  let excludedTowns = [];

  // Town fixture rows carry the geo projection inline (total/totalCapped/
  // applications), so `getGeo` is a pure lookup — no network in fixture mode.
  if (townFixturePath) {
    const rawTowns = await readFile(townFixturePath, 'utf-8');
    const townEntries = JSON.parse(rawTowns);
    if (!Array.isArray(townEntries)) {
      throw new Error(`town fixture ${townFixturePath} must be a JSON array`);
    }
    const authorities = await loadAuthorities();
    ({ publishedTowns, excludedTowns } = await renderTownPages({
      outDir,
      towns: townEntries,
      authorities,
      getGeo: async (town) => ({
        applications: Array.isArray(town.applications) ? town.applications : [],
        total: town.total,
        totalCapped: Boolean(town.totalCapped),
      }),
      limit,
      logger,
    }));
  }

  await writeFile(
    join(outDir, 'sitemap.xml'),
    renderSitemap([...published, ...publishedTowns]),
    'utf-8',
  );
  return { skipped: false, published, excluded, publishedTowns, excludedTowns };
}

/**
 * @param {Object} args
 * @param {string} args.outDir
 * @param {string} args.apiBase
 * @param {string} args.buildKey
 * @param {number} args.limit
 * @param {typeof globalThis.fetch} args.fetchImpl
 * @param {() => Promise<Array<{ id: number, name: string, areaType: string }>>} args.loadAuthorities
 * @param {() => Promise<Town[]>} args.loadTowns
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<PrerenderResult>}
 */
async function runLiveMode(args) {
  const {
    outDir,
    apiBase,
    buildKey,
    limit,
    fetchImpl,
    loadAuthorities,
    loadTowns,
    logger,
  } = args;

  const authorities = await loadAuthorities();

  /** @type {string[]} */
  const published = [];
  /** @type {Array<{ name: string, reason: string }>} */
  const excluded = [];
  const seenSlugs = new Set();

  for (const authority of authorities) {
    if (!isQualifyingAreaType(authority.areaType)) {
      excluded.push({ name: authority.name, reason: 'areaType' });
      continue;
    }
    const recent = await fetchRecentApplications(
      apiBase,
      authority.id,
      buildKey,
      limit,
      fetchImpl,
    );
    await considerAuthority({
      outDir,
      authorityId: authority.id,
      name: authority.name,
      areaType: authority.areaType,
      areaName: recent.areaName || authority.name,
      total: recent.total,
      totalCapped: recent.totalCapped,
      applications: recent.applications,
      limit,
      published,
      excluded,
      seenSlugs,
      logger,
    });
  }

  // Town pages: one bounded geo read per town, scoped to its authority partition
  // and centroid. Reuses the same authority list (no extra HTTP) to resolve slugs.
  const towns = await loadTowns();
  const { publishedTowns, excludedTowns } = await renderTownPages({
    outDir,
    towns,
    authorities,
    getGeo: (town) => fetchRecentNearby(apiBase, town, buildKey, limit, fetchImpl),
    limit,
    logger,
  });

  await writeFile(
    join(outDir, 'sitemap.xml'),
    renderSitemap([...published, ...publishedTowns]),
    'utf-8',
  );
  return { skipped: false, published, excluded, publishedTowns, excludedTowns };
}

/**
 * Orchestrate the prerender. Dependency-injected for testing.
 *
 * @param {Object} options
 * @param {string} options.outDir                  directory to emit into (web/dist)
 * @param {string} [options.apiBase]               API base URL (live mode)
 * @param {string} [options.buildKey]              SITE_BUILD_KEY (live mode)
 * @param {string} [options.fixturePath]           committed authority JSON fixture (fixture mode)
 * @param {string} [options.townFixturePath]       committed town JSON fixture (fixture mode)
 * @param {number} [options.limit]                 applications rendered per page
 * @param {typeof globalThis.fetch} [options.fetchImpl]
 * @param {() => Promise<Array<{ id: number, name: string, areaType: string }>>} [options.loadAuthorities]
 * @param {() => Promise<Town[]>} [options.loadTowns]
 * @param {{ log: Function, warn: Function, error: Function }} [options.logger]
 * @returns {Promise<PrerenderResult>}
 */
export async function runPrerender(options) {
  const {
    outDir,
    apiBase,
    buildKey,
    fixturePath,
    townFixturePath,
    limit = DEFAULT_LIMIT,
    fetchImpl = globalThis.fetch,
    loadAuthorities = () => loadAuthoritiesFromFile(AUTHORITIES_FILE, readFile),
    loadTowns = () => loadTownsFromFile(TOWNS_FILE, readFile),
    logger = console,
  } = options;

  if (fixturePath || townFixturePath) {
    return runFixtureMode({
      outDir,
      fixturePath,
      townFixturePath,
      limit,
      loadAuthorities,
      logger,
    });
  }

  if (!buildKey) {
    return {
      skipped: true,
      reason: 'SITE_BUILD_KEY not set',
      published: [],
      excluded: [],
      publishedTowns: [],
      excludedTowns: [],
    };
  }

  if (!apiBase) {
    throw new Error(
      'SITE_BUILD_KEY is set but no API base URL is configured (PRERENDER_API_BASE / VITE_API_BASE_URL)',
    );
  }

  return runLiveMode({
    outDir,
    apiBase: trimTrailingSlash(apiBase),
    buildKey,
    limit,
    fetchImpl,
    loadAuthorities,
    loadTowns,
    logger,
  });
}

/**
 * CLI entry point. Reads configuration from the environment and writes into
 * web/dist (resolved relative to this script, independent of cwd).
 *
 * @returns {Promise<void>}
 */
async function main() {
  const outDir = process.env.PRERENDER_OUT_DIR || join(SCRIPT_DIR, '..', 'dist');
  const apiBase =
    process.env.PRERENDER_API_BASE || process.env.VITE_API_BASE_URL;
  const buildKey = process.env.SITE_BUILD_KEY;
  const fixturePath = process.env.PRERENDER_FIXTURE;
  const townFixturePath = process.env.PRERENDER_TOWN_FIXTURE;

  try {
    const result = await runPrerender({
      outDir,
      apiBase,
      buildKey,
      fixturePath,
      townFixturePath,
    });
    if (result.skipped) {
      console.log(`[prerender] skipped — ${result.reason} (SPA build only)`);
      return;
    }
    console.log(
      `[prerender] wrote ${result.published.length} authority page(s) and ` +
        `${result.publishedTowns.length} town page(s); excluded ` +
        `${result.excluded.length} authority/-ies and ` +
        `${result.excludedTowns.length} town(s)`,
    );
  } catch (err) {
    console.error(
      `[prerender] FAILED: ${err instanceof Error ? err.message : String(err)}`,
    );
    process.exitCode = 1;
  }
}

// Run main() only when invoked directly (`node scripts/prerender-planning.mjs`),
// not when imported by tests.
if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  await main();
}
