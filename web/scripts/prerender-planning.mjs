/**
 * Static prerender for the programmatic authority-level SEO pages
 * (`/planning/<slug>`). Runs AFTER `vite build`, emitting fully static,
 * hydration-free HTML into `web/dist/planning/<slug>/index.html` plus a
 * `web/dist/sitemap.xml`.
 *
 * Default (no-flag) run modes, selected by environment — UNCHANGED, this is what
 * `npm run build` invokes:
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
 * Two opt-in, argv-selected modes split data-fetching from rendering so the
 * large SEO corpus can be refreshed weekly (one API hit) and rebuilt every
 * release offline (zero API hits) — see GH #598 / ADR for the blob-snapshot
 * decision:
 *   - `--fetch`   -> snapshot: same live data-gathering as the live mode above,
 *                    but serialise every render input into a single
 *                    `seo-snapshot.json` (path: PRERENDER_SNAPSHOT, default
 *                    web/seo-snapshot.json). Emits NO HTML. The ONLY API caller.
 *   - `--render`  -> offline: read `seo-snapshot.json` and rebuild every
 *                    `/planning/*` page + the sitemap with ZERO network calls.
 * Both reuse the exact page-generation pipeline; only the data source differs.
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
import { isSameNameAsAuthority } from './lib/same-name.mjs';
import { mergeRedirects } from './lib/redirect-config.mjs';

const DEFAULT_LIMIT = 30;

/**
 * Fallback centroid radius (metres) sent to `/v1/applications/near` for BOTH
 * the primary town point and every sibling centroid (tc-s0yf, GH #819),
 * mirroring the Go server's own default (`api-go/internal/applications/near.go`).
 * The committed town gazetteer (`web/src/data/towns.json`) has no per-town
 * radius field yet, so every town — primary and sibling alike — uses this one
 * value; the server's own clamp (max 10000m) never engages while every radius
 * sent equals its own default. Once the gazetteer grows a per-town radius,
 * this constant is replaced by that data.
 *
 * @type {number}
 */
const DEFAULT_NEAR_RADIUS_METERS = 5000;

/**
 * Current on-disk schema version of `seo-snapshot.json`. Bump when the snapshot
 * shape changes incompatibly so a stale snapshot can be detected/rejected.
 * @type {number}
 */
const SNAPSHOT_VERSION = 1;

/**
 * Derive a page's content `lastmod` from the applications it actually shows: the
 * maximum (freshest) `lastDifferent` of the rendered set. This is the honest
 * freshness signal for the sitemap — NOT the build clock, which would falsely
 * bump every unchanged page on every rebuild. Returns `undefined` when no shown
 * application carries a parseable date, so the caller can omit `<lastmod>` rather
 * than stamp an invalid/empty one.
 *
 * @param {ReadonlyArray<{ lastDifferent?: string | null }>} applications the
 *   rendered (sliced) application set
 * @returns {string | undefined} the max ISO `lastDifferent`, or undefined
 */
function maxLastmod(applications) {
  let max;
  let maxMs = -Infinity;
  for (const app of applications) {
    const iso = app?.lastDifferent;
    if (typeof iso !== 'string' || iso === '') {
      continue;
    }
    const ms = new Date(iso).getTime();
    if (!Number.isNaN(ms) && ms > maxMs) {
      maxMs = ms;
      max = iso;
    }
  }
  return max;
}

/**
 * Default published-population threshold when `SEO_TOWN_MIN_POPULATION` is unset,
 * empty, or not a positive finite integer. A town ships only if its built-up-area
 * population is at least this value (the ≥10 in-radius coverage gate applies on
 * top). The threshold is a build-time config knob so coverage can be ramped by
 * bumping the variable and rebuilding — no gazetteer regeneration.
 *
 * @type {number}
 */
export const DEFAULT_MIN_POPULATION = 20000;

/** Directory containing this script (`web/scripts`), independent of cwd. */
const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));

/**
 * Resolve the build-time town population threshold from the environment.
 * Reads `SEO_TOWN_MIN_POPULATION` and parses it to a positive finite integer,
 * falling back to {@link DEFAULT_MIN_POPULATION} when the value is missing,
 * empty/whitespace, non-numeric, or not strictly positive. A fractional value is
 * truncated toward zero (so "12345.9" -> 12345).
 *
 * @param {Record<string, string | undefined>} [env]
 * @returns {number} the resolved minimum population (always a positive integer)
 */
export function resolveMinPopulation(env = {}) {
  const raw = env.SEO_TOWN_MIN_POPULATION;
  if (typeof raw !== 'string' || raw.trim().length === 0) {
    return DEFAULT_MIN_POPULATION;
  }
  const parsed = Number.parseInt(raw.trim(), 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return DEFAULT_MIN_POPULATION;
  }
  return parsed;
}

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
 * time. Each row is `{ slug, name, lat, lng, authorityId, population }`.
 * @type {string}
 */
export const TOWNS_FILE = join(SCRIPT_DIR, '..', 'src', 'data', 'towns.json');

/**
 * The hand-written base Static Web Apps config (`navigationFallback`,
 * `globalHeaders`, `routes`). Vite copies `public/` -> `dist/` BEFORE the
 * prerender runs, so this is also present in the outDir; we deliberately read
 * the SOURCE here (so re-running the prerender never double-merges) and write the
 * merged result — base routes plus the same-name 301 redirects — into the outDir.
 * @type {string}
 */
export const BASE_SWA_CONFIG_FILE = join(
  SCRIPT_DIR,
  '..',
  'public',
  'staticwebapp.config.json',
);

/**
 * @typedef {import('./lib/render-sitemap.mjs').SitemapEntry} SitemapEntry
 */

/**
 * @typedef {Object} PrerenderResult
 * @property {boolean} skipped
 * @property {string} [reason]
 * @property {string[]} published                       authority slugs written
 * @property {Array<{ name: string, reason: string }>} excluded
 * @property {string[]} publishedTowns                  town paths written (<authority>/<town>)
 * @property {Array<{ name: string, reason: string }>} excludedTowns
 * @property {string[]} [redirects]                     suppressed same-name town paths 301'd to their authority page
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
 * @property {number} population    built-up-area population (gates which towns ship)
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
      !Number.isFinite(t?.authorityId) ||
      !Number.isFinite(t?.population)
    ) {
      throw new Error(`town gazetteer at ${filePath} has a malformed town row`);
    }
  }
  return list;
}

/**
 * Group the FULL committed gazetteer by authorityId, once per build — every
 * town in an authority is a Voronoi sibling candidate for every OTHER town in
 * that same authority (tc-s0yf, GH #819), regardless of whether it
 * individually clears the population/coverage gates. Grouped from the
 * unfiltered town list (not `eligibleTowns`) so a below-threshold town still
 * contributes its centroid to a neighbour's partition.
 *
 * @param {ReadonlyArray<Town>} towns
 * @returns {Map<number, Town[]>}
 */
function groupTownsByAuthority(towns) {
  /** @type {Map<number, Town[]>} */
  const map = new Map();
  for (const town of towns) {
    const group = map.get(town.authorityId);
    if (group) {
      group.push(town);
    } else {
      map.set(town.authorityId, [town]);
    }
  }
  return map;
}

/**
 * Every OTHER gazetteer town sharing `town`'s authorityId — the sibling
 * centroids passed to `/v1/applications/near` for the town-level Voronoi
 * partition. Identified by slug (unique within an authority — it forms half
 * of each town's page path) rather than object identity, so it works
 * regardless of how `gazetteerByAuthority` was built.
 *
 * @param {Town} town
 * @param {Map<number, Town[]>} gazetteerByAuthority
 * @returns {Town[]}
 */
function siblingTownsOf(town, gazetteerByAuthority) {
  const group = gazetteerByAuthority.get(town.authorityId) ?? [];
  return group.filter((t) => t.slug !== town.slug);
}

/**
 * Serialize each sibling town centroid as a repeated `sibling=lat,lng,radius`
 * query-string suffix for `/v1/applications/near` (tc-s0yf, GH #819) — one per
 * OTHER gazetteer town sharing the primary town's authority. Every sibling
 * uses {@link DEFAULT_NEAR_RADIUS_METERS} for its radius component (no
 * per-town radius data exists yet — see the {@link Town} typedef). Returns
 * `''` (no params at all) when `siblings` is empty, e.g. a town that is the
 * only gazetteer entry in its authority.
 *
 * @param {ReadonlyArray<Town>} siblings
 * @returns {string}
 */
function siblingQueryParams(siblings) {
  return siblings
    .map((s) => `&sibling=${s.lat},${s.lng},${DEFAULT_NEAR_RADIUS_METERS}`)
    .join('');
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
 * @returns {Promise<{ areaName: string, applications: object[], total: number, statusBreakdown: object[] }>}
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
    !Array.isArray(body.statusBreakdown)
  ) {
    throw new Error(
      `GET /v1/authorities/${authorityId}/applications returned an unexpected shape`,
    );
  }
  return {
    areaName: typeof body.areaName === 'string' ? body.areaName : '',
    applications: body.applications,
    total: body.total,
    statusBreakdown: body.statusBreakdown,
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
 * Read the hand-written base SWA config, merge a 301 redirect for every
 * suppressed same-name town page (tc-77ll), and write the result into the build
 * outDir. Without this, a suppressed `/planning/<x>/<x>` URL would fall through
 * `navigationFallback` to `index.html` (a 200 SPA shell = soft-404, worse than
 * the duplicate it replaced). The merged route set stays well within the SWA
 * route-count limit (≤151 redirects plus a handful of base routes).
 *
 * @param {Object} args
 * @param {string} args.outDir
 * @param {ReadonlyArray<string>} args.redirects   suppressed town paths ("<auth>/<town>")
 * @param {string} args.baseConfigPath             source base config (web/public/staticwebapp.config.json)
 * @returns {Promise<void>}
 */
async function writeRedirectConfig({ outDir, redirects, baseConfigPath }) {
  const raw = await readFile(baseConfigPath, 'utf-8');
  const baseConfig = JSON.parse(raw);
  const merged = mergeRedirects(baseConfig, redirects);
  await writeFile(
    join(outDir, 'staticwebapp.config.json'),
    `${JSON.stringify(merged, null, 2)}\n`,
    'utf-8',
  );
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
 * @param {object[]} args.statusBreakdown
 * @param {object[]} args.applications
 * @param {number} args.limit
 * @param {string[]} args.published
 * @param {SitemapEntry[]} args.sitemapEntries           parallel { path, lastmod } records
 * @param {Array<{ name: string, reason: string }>} args.excluded
 * @param {Set<string>} args.seenSlugs
 * @param {Map<number, Array<{ name: string, slug: string }>>} args.townsByAuthority
 *   published towns keyed by parent authorityId, accumulated by the (earlier)
 *   town pass; the authority page links down to its own entry (sorted by name).
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<void>}
 */
async function considerAuthority(args) {
  const {
    outDir,
    authorityId,
    name,
    total,
    statusBreakdown,
    applications,
    areaName,
    limit,
    published,
    sitemapEntries,
    excluded,
    seenSlugs,
    townsByAuthority,
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

  // Authority data already arrives recency-ordered (the bounded read sorts by
  // lastDifferent DESC), so no re-sort here — only the town path needs that.
  const shown = applications.slice(0, limit);
  // Only this authority's PUBLISHED towns (gated-in by the earlier town pass) are
  // linked, sorted by display name, so the page never links to a 404.
  const towns = [...(townsByAuthority?.get(authorityId) ?? [])].sort((a, b) =>
    a.name.localeCompare(b.name),
  );
  await writePage(outDir, {
    slug,
    areaName: areaName || name,
    authorityId,
    total,
    statusBreakdown,
    applications: shown,
    towns,
  });
  published.push(slug);
  sitemapEntries.push({ path: slug, lastmod: maxLastmod(shown) });
}

/**
 * Fetch the bounded recent-applications-near-a-point projection for one town via
 * the build-key-gated geo endpoint. Scopes the spatial query to the town's
 * authority partition (authorityId) and centroid (lat/lng), passing every OTHER
 * same-authority gazetteer town as a `sibling` centroid so the server can run
 * its town-level Voronoi partition (tc-s0yf, GH #819) and return ONLY the
 * applications assigned to THIS town — already ordered by
 * `GREATEST(decidedDate, startDate) DESC`. No client-side re-sort or
 * re-partition is needed (`order=distance` is gone). Throws on any non-OK
 * status or unexpected shape.
 *
 * @param {string} apiBase
 * @param {Town} town
 * @param {ReadonlyArray<Town>} siblings   every OTHER gazetteer town sharing `town.authorityId`
 * @param {string} buildKey
 * @param {number} limit
 * @param {typeof globalThis.fetch} fetchImpl
 * @returns {Promise<{ applications: object[], total: number, statusBreakdown: object[] }>}
 */
async function fetchRecentNearby(
  apiBase,
  town,
  siblings,
  buildKey,
  limit,
  fetchImpl,
) {
  const url =
    `${apiBase}/v1/applications/near?authorityId=${town.authorityId}` +
    `&lat=${town.lat}&lng=${town.lng}&radius=${DEFAULT_NEAR_RADIUS_METERS}` +
    `&limit=${limit}${siblingQueryParams(siblings)}`;
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
    !Array.isArray(body.statusBreakdown)
  ) {
    throw new Error(
      `GET /v1/applications/near (authority ${town.authorityId}, ${town.name}) ` +
        `returned an unexpected shape`,
    );
  }
  return {
    applications: body.applications,
    total: body.total,
    statusBreakdown: body.statusBreakdown,
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
 * @param {object[]} args.statusBreakdown
 * @param {object[]} args.applications
 * @param {number} args.limit
 * @param {string[]} args.publishedTowns
 * @param {SitemapEntry[]} args.sitemapEntries           parallel { path, lastmod } records
 * @param {Array<{ name: string, reason: string }>} args.excludedTowns
 * @param {string[]} args.redirects
 *   suppressed same-name town paths to 301-redirect to their authority page
 * @param {Set<string>} args.seenPaths
 * @param {Map<number, Array<{ name: string, slug: string }>>} args.townsByAuthority
 *   accumulates each published town under its parent authorityId so the (later)
 *   authority pass can link down to its town children.
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<void>}
 */
async function considerTown(args) {
  const {
    outDir,
    town,
    authorities,
    total,
    statusBreakdown,
    applications,
    limit,
    publishedTowns,
    sitemapEntries,
    excludedTowns,
    redirects,
    seenPaths,
    townsByAuthority,
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

  // Same-name dedup (tc-77ll / #717): a town whose NORMALIZED slug equals its
  // authority's slug (e.g. /planning/wrexham/wrexham vs /planning/wrexham)
  // duplicates the stronger authority page with an identical <title>. Suppress
  // it — no page, no sitemap entry, and (by returning before townsByAuthority is
  // touched) no self-link from the authority page — and record a 301 to the
  // authority page so the dead, likely-indexed URL is not a soft-404. Only
  // reached after the coverage gate, so a same-name town that never published
  // gets no redirect.
  if (isSameNameAsAuthority(authorityName, town.slug)) {
    excludedTowns.push({ name: town.name, reason: 'same-name' });
    redirects.push(townPagePath(authoritySlug, town.slug));
    return;
  }

  const path = townPagePath(authoritySlug, town.slug);
  if (seenPaths.has(path)) {
    logger.warn(`[prerender] duplicate town path "${path}" — skipped`);
    excludedTowns.push({ name: town.name, reason: 'duplicate-path' });
    return;
  }
  seenPaths.add(path);

  // The near read is a fully server-side town-level Voronoi partition, already
  // ordered by GREATEST(decidedDate, startDate) DESC (tc-s0yf) — like the
  // authority read (considerAuthority, above), so no re-sort here.
  const shown = applications.slice(0, limit);
  await writeTownPage(outDir, {
    townName: town.name,
    townSlug: town.slug,
    authorityName,
    authoritySlug,
    authorityId: town.authorityId,
    total,
    statusBreakdown,
    applications: shown,
  });
  publishedTowns.push(path);
  sitemapEntries.push({ path, lastmod: maxLastmod(shown) });
  // Record this published town under its parent authority so the authority page
  // can link down to it. Only reached after the coverage gate + dedup pass, so a
  // gated-out or duplicate town is never linked.
  if (townsByAuthority) {
    const siblings = townsByAuthority.get(town.authorityId);
    if (siblings) {
      siblings.push({ name: town.name, slug: town.slug });
    } else {
      townsByAuthority.set(town.authorityId, [
        { name: town.name, slug: town.slug },
      ]);
    }
  }
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
 * @param {(town: Town) => Promise<{ applications: object[], total: number, statusBreakdown: object[] }>} args.getGeo
 * @param {number} args.limit
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<{ publishedTowns: string[], townSitemapEntries: SitemapEntry[], excludedTowns: Array<{ name: string, reason: string }>, townsByAuthority: Map<number, Array<{ name: string, slug: string }>>, redirects: string[] }>}
 */
async function renderTownPages(args) {
  const { outDir, towns, authorities, getGeo, limit, logger } = args;

  /** @type {string[]} */
  const publishedTowns = [];
  /** @type {SitemapEntry[]} */
  const townSitemapEntries = [];
  /** @type {Array<{ name: string, reason: string }>} */
  const excludedTowns = [];
  // Suppressed same-name town paths (tc-77ll), to be 301'd to their authority.
  /** @type {string[]} */
  const redirects = [];
  const seenPaths = new Set();
  // Published towns keyed by parent authorityId — handed to the authority pass so
  // each authority page can link down to its own (gated-in) town children.
  /** @type {Map<number, Array<{ name: string, slug: string }>>} */
  const townsByAuthority = new Map();

  for (const town of towns) {
    const geo = await getGeo(town);
    await considerTown({
      outDir,
      town,
      authorities,
      total: geo.total,
      statusBreakdown: Array.isArray(geo.statusBreakdown)
        ? geo.statusBreakdown
        : [],
      applications: Array.isArray(geo.applications) ? geo.applications : [],
      limit,
      publishedTowns,
      sitemapEntries: townSitemapEntries,
      excludedTowns,
      redirects,
      seenPaths,
      townsByAuthority,
      logger,
    });
  }

  return {
    publishedTowns,
    townSitemapEntries,
    excludedTowns,
    townsByAuthority,
    redirects,
  };
}

/**
 * Render an authority-entry set and a town-entry set into static pages plus a
 * sitemap, sharing the EXACT page-generation pipeline every mode uses
 * (`considerAuthority`, `renderTownPages`, `renderSitemap` — the coverage gate,
 * slug/path dedup, recency sort and lastmod all live inside those helpers). The
 * entries carry their projection data inline, so this is pure: it performs NO
 * network I/O. Both fixture mode and offline `--render` mode feed it; the only
 * thing that differs upstream is where the entries came from (a committed
 * fixture vs. a fetched snapshot).
 *
 * @param {Object} args
 * @param {string} args.outDir
 * @param {ReadonlyArray<{ id: number, name: string, areaType: string, areaName?: string, total: number, statusBreakdown?: object[], applications?: object[] }>} args.authorityEntries
 * @param {ReadonlyArray<Town & { total: number, statusBreakdown?: object[], applications?: object[] }>} args.townEntries
 * @param {ReadonlyArray<{ id: number, name: string }>} args.authorities  resolves a town's parent-authority slug
 * @param {number} args.limit
 * @param {string} [args.baseConfigPath]  base SWA config to merge same-name 301s into
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<PrerenderResult>}
 */
async function renderEntries(args) {
  const {
    outDir,
    authorityEntries,
    townEntries,
    authorities,
    limit,
    baseConfigPath = BASE_SWA_CONFIG_FILE,
    logger,
  } = args;

  // Town pass FIRST: an authority page must link only to towns that actually
  // published (cleared the coverage gate), so the per-authority town map has to
  // be built before authority pages render. Both passes are independent; sitemap
  // order is immaterial. Town entries carry the geo projection inline, so
  // `getGeo` is a pure lookup — no network. With zero town entries the loop is a
  // no-op and `authorities` is never consulted.
  const {
    publishedTowns,
    townSitemapEntries,
    excludedTowns,
    townsByAuthority,
    redirects,
  } = await renderTownPages({
    outDir,
    towns: townEntries,
    authorities,
    getGeo: async (town) => ({
      applications: Array.isArray(town.applications) ? town.applications : [],
      total: town.total,
      statusBreakdown: Array.isArray(town.statusBreakdown)
        ? town.statusBreakdown
        : [],
    }),
    limit,
    logger,
  });

  /** @type {string[]} */
  const published = [];
  /** @type {SitemapEntry[]} */
  const authoritySitemapEntries = [];
  /** @type {Array<{ name: string, reason: string }>} */
  const excluded = [];
  const seenSlugs = new Set();

  for (const entry of authorityEntries) {
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
      statusBreakdown: Array.isArray(entry.statusBreakdown)
        ? entry.statusBreakdown
        : [],
      applications: Array.isArray(entry.applications) ? entry.applications : [],
      limit,
      published,
      sitemapEntries: authoritySitemapEntries,
      excluded,
      seenSlugs,
      townsByAuthority,
      logger,
    });
  }

  await writeFile(
    join(outDir, 'sitemap.xml'),
    renderSitemap([...authoritySitemapEntries, ...townSitemapEntries]),
    'utf-8',
  );
  await writeRedirectConfig({ outDir, redirects, baseConfigPath });
  return {
    skipped: false,
    published,
    excluded,
    publishedTowns,
    excludedTowns,
    redirects,
  };
}

/**
 * @param {Object} args
 * @param {string} args.outDir
 * @param {string} args.fixturePath               authority fixture (optional)
 * @param {string} [args.townFixturePath]         town fixture (optional)
 * @param {number} args.limit
 * @param {string} [args.baseConfigPath]          base SWA config for same-name 301s
 * @param {() => Promise<Array<{ id: number, name: string, areaType: string }>>} args.loadAuthorities
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<PrerenderResult>}
 */
async function runFixtureMode(args) {
  const {
    outDir,
    fixturePath,
    townFixturePath,
    limit,
    baseConfigPath,
    loadAuthorities,
    logger,
  } = args;

  /** @type {Array<{ id: number, name: string, areaType: string }>} */
  let authorityEntries = [];
  if (fixturePath) {
    const raw = await readFile(fixturePath, 'utf-8');
    authorityEntries = JSON.parse(raw);
    if (!Array.isArray(authorityEntries)) {
      throw new Error(`fixture ${fixturePath} must be a JSON array`);
    }
  }

  /** @type {Town[]} */
  let townEntries = [];
  /** @type {Array<{ id: number, name: string }>} */
  let authorities = [];
  // The authority list is only needed to resolve town slugs, so it's loaded only
  // when there is a town fixture — preserving the original behaviour exactly.
  if (townFixturePath) {
    const rawTowns = await readFile(townFixturePath, 'utf-8');
    townEntries = JSON.parse(rawTowns);
    if (!Array.isArray(townEntries)) {
      throw new Error(`town fixture ${townFixturePath} must be a JSON array`);
    }
    authorities = await loadAuthorities();
  }

  return renderEntries({
    outDir,
    authorityEntries,
    townEntries,
    authorities,
    limit,
    baseConfigPath,
    logger,
  });
}

/**
 * @param {Object} args
 * @param {string} args.outDir
 * @param {string} args.apiBase
 * @param {string} args.buildKey
 * @param {number} args.limit
 * @param {number} args.minPopulation
 * @param {string} [args.baseConfigPath]  base SWA config for same-name 301s
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
    minPopulation,
    baseConfigPath = BASE_SWA_CONFIG_FILE,
    fetchImpl,
    loadAuthorities,
    loadTowns,
    logger,
  } = args;

  const authorities = await loadAuthorities();

  // Town pass FIRST: the authority pages link down to their PUBLISHED towns, so
  // the per-authority town map must be built before authority pages render.
  // One bounded geo read per town, scoped to its authority partition and
  // centroid. Reuses the same authority list (no extra HTTP) to resolve slugs.
  const towns = await loadTowns();

  // Grouped from the FULL, unfiltered gazetteer (tc-s0yf) — every town in an
  // authority is a Voronoi sibling candidate for every other town in that
  // authority, regardless of the population gate applied below.
  const gazetteerByAuthority = groupTownsByAuthority(towns);

  // Population threshold gate, applied BEFORE the per-town geo fetch so that
  // below-threshold towns never even hit the API. The threshold is a build-time
  // config value (SEO_TOWN_MIN_POPULATION, default 20000); ramping coverage means
  // bumping the variable and rebuilding — no gazetteer regeneration. The ≥10
  // in-radius coverage gate (inside `considerTown`) still applies on top.
  /** @type {Town[]} */
  const eligibleTowns = [];
  /** @type {Array<{ name: string, reason: string }>} */
  const populationExcludedTowns = [];
  for (const town of towns) {
    if (town.population >= minPopulation) {
      eligibleTowns.push(town);
    } else {
      populationExcludedTowns.push({ name: town.name, reason: 'population' });
    }
  }

  const {
    publishedTowns,
    townSitemapEntries,
    excludedTowns,
    townsByAuthority,
    redirects,
  } = await renderTownPages({
    outDir,
    towns: eligibleTowns,
    authorities,
    getGeo: (town) =>
      fetchRecentNearby(
        apiBase,
        town,
        siblingTownsOf(town, gazetteerByAuthority),
        buildKey,
        limit,
        fetchImpl,
      ),
    limit,
    logger,
  });
  excludedTowns.push(...populationExcludedTowns);

  /** @type {string[]} */
  const published = [];
  /** @type {SitemapEntry[]} */
  const authoritySitemapEntries = [];
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
      statusBreakdown: recent.statusBreakdown,
      applications: recent.applications,
      limit,
      published,
      sitemapEntries: authoritySitemapEntries,
      excluded,
      seenSlugs,
      townsByAuthority,
      logger,
    });
  }

  await writeFile(
    join(outDir, 'sitemap.xml'),
    renderSitemap([...authoritySitemapEntries, ...townSitemapEntries]),
    'utf-8',
  );
  await writeRedirectConfig({ outDir, redirects, baseConfigPath });
  return {
    skipped: false,
    published,
    excluded,
    publishedTowns,
    excludedTowns,
    redirects,
  };
}

/**
 * @typedef {Object} SeoSnapshot
 * @property {number} version
 * @property {string} generatedAt                          ISO timestamp the snapshot was gathered
 * @property {number} minPopulation                        the town population cut applied at fetch
 * @property {number} limit                                applications fetched/rendered per page
 * @property {Array<{ id: number, name: string }>} authorities           FULL list (id+name), for offline slug resolution
 * @property {Array<{ id: number, name: string, areaType: string, areaName: string, total: number, statusBreakdown: object[], applications: object[] }>} authorityPages   one per qualifying authority
 * @property {Array<Town & { total: number, statusBreakdown: object[], applications: object[] }>} townPages                one per population-eligible town
 */

/**
 * Gather every authority/town render input from the live API into a single
 * self-contained snapshot — the SAME data-gathering as {@link runLiveMode}
 * (qualifying authorities + population-eligible towns via the build-key-gated
 * endpoints, with the same areaType and population pre-filters), but collecting
 * the projections instead of writing pages. The coverage gate and slug/path
 * dedup are deliberately NOT applied here: they are page-generation decisions
 * re-applied at render time, so the snapshot carries the full fetched set and a
 * `--render` reproduces today's exact page set. Fails LOUD on any
 * transport/shape error (the per-endpoint fetch helpers throw).
 *
 * @param {Object} args
 * @param {string} args.apiBase
 * @param {string} args.buildKey
 * @param {number} args.limit
 * @param {number} args.minPopulation
 * @param {typeof globalThis.fetch} args.fetchImpl
 * @param {() => Promise<Array<{ id: number, name: string, areaType: string }>>} args.loadAuthorities
 * @param {() => Promise<Town[]>} args.loadTowns
 * @param {() => string} args.now
 * @returns {Promise<SeoSnapshot>}
 */
async function gatherSnapshot(args) {
  const {
    apiBase,
    buildKey,
    limit,
    minPopulation,
    fetchImpl,
    loadAuthorities,
    loadTowns,
    now,
  } = args;

  const authorities = await loadAuthorities();

  /** @type {SeoSnapshot['authorityPages']} */
  const authorityPages = [];
  for (const authority of authorities) {
    // areaType pre-filter: non-qualifying authorities are never fetched (same as
    // live mode), so they never enter the snapshot.
    if (!isQualifyingAreaType(authority.areaType)) {
      continue;
    }
    const recent = await fetchRecentApplications(
      apiBase,
      authority.id,
      buildKey,
      limit,
      fetchImpl,
    );
    authorityPages.push({
      id: authority.id,
      name: authority.name,
      areaType: authority.areaType,
      areaName: recent.areaName || authority.name,
      total: recent.total,
      statusBreakdown: recent.statusBreakdown,
      applications: recent.applications,
    });
  }

  const towns = await loadTowns();

  // Grouped from the FULL, unfiltered gazetteer (tc-s0yf) — every town in an
  // authority is a Voronoi sibling candidate for every other town in that
  // authority, regardless of the population gate applied below.
  const gazetteerByAuthority = groupTownsByAuthority(towns);

  /** @type {SeoSnapshot['townPages']} */
  const townPages = [];
  for (const town of towns) {
    // population pre-filter, applied BEFORE the geo fetch (same as live mode), so
    // below-threshold towns never hit the API and never enter the snapshot.
    if (town.population < minPopulation) {
      continue;
    }
    const geo = await fetchRecentNearby(
      apiBase,
      town,
      siblingTownsOf(town, gazetteerByAuthority),
      buildKey,
      limit,
      fetchImpl,
    );
    townPages.push({
      slug: town.slug,
      name: town.name,
      lat: town.lat,
      lng: town.lng,
      authorityId: town.authorityId,
      population: town.population,
      total: geo.total,
      statusBreakdown: geo.statusBreakdown,
      applications: geo.applications,
    });
  }

  return {
    version: SNAPSHOT_VERSION,
    generatedAt: now(),
    minPopulation,
    limit,
    // The FULL authority list (id+name only) — town pages resolve their parent
    // authority slug from this offline, including non-qualifying parents that
    // never get an authority page of their own.
    authorities: authorities.map((a) => ({ id: a.id, name: a.name })),
    authorityPages,
    townPages,
  };
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
 * @param {Record<string, string | undefined>} [options.env]  environment (for SEO_TOWN_MIN_POPULATION)
 * @param {string} [options.baseConfigPath]        base SWA config to merge same-name 301s into
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
    env = process.env,
    baseConfigPath = BASE_SWA_CONFIG_FILE,
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
      baseConfigPath,
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
      redirects: [],
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
    minPopulation: resolveMinPopulation(env),
    baseConfigPath,
    fetchImpl,
    loadAuthorities,
    loadTowns,
    logger,
  });
}

/**
 * Snapshot mode (`--fetch`). Gathers the live render inputs for every qualifying
 * authority and population-eligible town and writes a single self-contained
 * `seo-snapshot.json`. Emits NO HTML. This is the ONLY mode that touches the Go
 * API; `--render` then rebuilds the pages offline from the snapshot. Because it
 * is always invoked deliberately (the weekly job), a missing key or base is a
 * hard error — never a silent skip.
 *
 * @param {Object} options
 * @param {string} options.snapshotPath              file to write the snapshot to
 * @param {string} [options.apiBase]                 API base URL (PRERENDER_API_BASE / VITE_API_BASE_URL)
 * @param {string} [options.buildKey]                SITE_BUILD_KEY
 * @param {number} [options.limit]                   applications fetched per page
 * @param {Record<string, string | undefined>} [options.env]  environment (for SEO_TOWN_MIN_POPULATION)
 * @param {typeof globalThis.fetch} [options.fetchImpl]
 * @param {() => Promise<Array<{ id: number, name: string, areaType: string }>>} [options.loadAuthorities]
 * @param {() => Promise<Town[]>} [options.loadTowns]
 * @param {() => string} [options.now]               clock seam for `generatedAt`
 * @param {{ log: Function, warn: Function, error: Function }} [options.logger]
 * @returns {Promise<SeoSnapshot>}
 */
export async function runFetch(options) {
  const {
    snapshotPath,
    apiBase,
    buildKey,
    limit = DEFAULT_LIMIT,
    env = process.env,
    fetchImpl = globalThis.fetch,
    loadAuthorities = () => loadAuthoritiesFromFile(AUTHORITIES_FILE, readFile),
    loadTowns = () => loadTownsFromFile(TOWNS_FILE, readFile),
    now = () => new Date().toISOString(),
    logger = console,
  } = options;

  if (!buildKey) {
    throw new Error('--fetch requires SITE_BUILD_KEY to be set');
  }
  if (!apiBase) {
    throw new Error(
      '--fetch requires an API base URL (PRERENDER_API_BASE / VITE_API_BASE_URL)',
    );
  }

  const snapshot = await gatherSnapshot({
    apiBase: trimTrailingSlash(apiBase),
    buildKey,
    limit,
    minPopulation: resolveMinPopulation(env),
    fetchImpl,
    loadAuthorities,
    loadTowns,
    now,
  });

  await mkdir(dirname(snapshotPath), { recursive: true });
  await writeFile(snapshotPath, JSON.stringify(snapshot), 'utf-8');
  logger.log?.(
    `[prerender] --fetch wrote ${snapshot.authorityPages.length} authority + ` +
      `${snapshot.townPages.length} town render input(s) to ${snapshotPath}`,
  );
  return snapshot;
}

/**
 * Validate a parsed snapshot loudly. A render must never silently emit an empty
 * or partial page set from a malformed snapshot, so anything that is not a
 * well-formed snapshot (the three required arrays) throws.
 *
 * @param {unknown} snapshot
 * @param {string} snapshotPath
 * @returns {asserts snapshot is SeoSnapshot}
 */
function assertValidSnapshot(snapshot, snapshotPath) {
  if (
    !snapshot ||
    typeof snapshot !== 'object' ||
    !Array.isArray(snapshot.authorities) ||
    !Array.isArray(snapshot.authorityPages) ||
    !Array.isArray(snapshot.townPages)
  ) {
    throw new Error(
      `SEO snapshot at ${snapshotPath} is missing one of the required arrays ` +
        `(authorities, authorityPages, townPages)`,
    );
  }
}

/**
 * Offline render mode (`--render`). Reads `seo-snapshot.json` from disk and
 * rebuilds every `/planning/*` page and the sitemap with ZERO network calls,
 * sharing the exact page-generation pipeline ({@link renderEntries}). Fails LOUD
 * if the snapshot is missing or malformed — no silent fallback to a live fetch,
 * which would re-introduce the per-release API load this split removes.
 *
 * @param {Object} options
 * @param {string} options.outDir
 * @param {string} options.snapshotPath
 * @param {number} [options.limit]                  override; defaults to the snapshot's own
 * @param {string} [options.baseConfigPath]         base SWA config to merge same-name 301s into
 * @param {(path: string, encoding: string) => Promise<string>} [options.readFileImpl]
 * @param {{ log: Function, warn: Function, error: Function }} [options.logger]
 * @returns {Promise<PrerenderResult>}
 */
export async function runRender(options) {
  const {
    outDir,
    snapshotPath,
    limit,
    baseConfigPath = BASE_SWA_CONFIG_FILE,
    readFileImpl = readFile,
    logger = console,
  } = options;

  let raw;
  try {
    raw = await readFileImpl(snapshotPath, 'utf-8');
  } catch (err) {
    throw new Error(
      `SEO snapshot not found at ${snapshotPath}: ` +
        `${err instanceof Error ? err.message : String(err)}`,
    );
  }

  let snapshot;
  try {
    snapshot = JSON.parse(raw);
  } catch (err) {
    throw new Error(
      `SEO snapshot at ${snapshotPath} is not valid JSON: ` +
        `${err instanceof Error ? err.message : String(err)}`,
    );
  }

  assertValidSnapshot(snapshot, snapshotPath);

  // The snapshot is self-contained: prefer its own limit so the rendered page
  // set matches exactly what was fetched; allow an explicit override.
  const renderLimit =
    typeof limit === 'number'
      ? limit
      : Number.isFinite(snapshot.limit)
        ? snapshot.limit
        : DEFAULT_LIMIT;

  return renderEntries({
    outDir,
    authorityEntries: snapshot.authorityPages,
    townEntries: snapshot.townPages,
    authorities: snapshot.authorities,
    limit: renderLimit,
    baseConfigPath,
    logger,
  });
}

/**
 * CLI entry point. Selects a mode from argv (`--fetch` / `--render`) or, with no
 * flags, falls back to the UNCHANGED env-driven behaviour that `npm run build`
 * relies on. Reads configuration from the environment and writes into web/dist
 * (resolved relative to this script, independent of cwd).
 *
 * @returns {Promise<void>}
 */
async function main() {
  const flags = process.argv.slice(2);
  const wantFetch = flags.includes('--fetch');
  const wantRender = flags.includes('--render');

  if (wantFetch && wantRender) {
    console.error('[prerender] FAILED: pass only one of --fetch / --render');
    process.exitCode = 1;
    return;
  }

  const snapshotPath =
    process.env.PRERENDER_SNAPSHOT || join(SCRIPT_DIR, '..', 'seo-snapshot.json');

  if (wantFetch) {
    const apiBase =
      process.env.PRERENDER_API_BASE || process.env.VITE_API_BASE_URL;
    const buildKey = process.env.SITE_BUILD_KEY;
    try {
      await runFetch({ snapshotPath, apiBase, buildKey });
    } catch (err) {
      console.error(
        `[prerender] --fetch FAILED: ${err instanceof Error ? err.message : String(err)}`,
      );
      process.exitCode = 1;
    }
    return;
  }

  if (wantRender) {
    const outDir =
      process.env.PRERENDER_OUT_DIR || join(SCRIPT_DIR, '..', 'dist');
    try {
      const result = await runRender({ outDir, snapshotPath });
      console.log(
        `[prerender] --render wrote ${result.published.length} authority page(s) ` +
          `and ${result.publishedTowns.length} town page(s) from ${snapshotPath} ` +
          `(zero network)`,
      );
    } catch (err) {
      console.error(
        `[prerender] --render FAILED: ${err instanceof Error ? err.message : String(err)}`,
      );
      process.exitCode = 1;
    }
    return;
  }

  // Default (no flags): the existing env-driven behaviour, UNCHANGED.
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
