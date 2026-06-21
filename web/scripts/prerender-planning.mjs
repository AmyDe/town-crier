/**
 * Static prerender for the programmatic authority-level SEO pages
 * (`/planning/<slug>`). Runs AFTER `vite build`, emitting fully static,
 * hydration-free HTML into `web/dist/planning/<slug>/index.html` plus a
 * `web/dist/sitemap.xml`.
 *
 * Three run modes, selected by environment:
 *   - SITE_BUILD_KEY set        -> live: fetch our gated API, fail LOUD on any
 *                                  transport/shape error (never empty pages).
 *   - PRERENDER_FIXTURE=<path>  -> fixture: render from a committed JSON file,
 *                                  no network (used by the tracer + tests).
 *   - neither set               -> skip: leave the SPA build untouched so
 *                                  `npm run build` still succeeds without a key.
 *
 * The build key is server-side only and is NEVER written into any page or the
 * client bundle. We read ONLY our own API — never PlanIt (ADR 0006).
 */

import { mkdir, writeFile, readFile } from 'node:fs/promises';
import { join, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { slugify } from './lib/slug.mjs';
import { isQualifyingAreaType } from './lib/area-types.mjs';
import { meetsCoverageGate } from './lib/coverage-gate.mjs';
import { renderPlanningPage } from './lib/render-page.mjs';
import { renderSitemap } from './lib/render-sitemap.mjs';

const DEFAULT_LIMIT = 30;

/**
 * @typedef {Object} PrerenderResult
 * @property {boolean} skipped
 * @property {string} [reason]
 * @property {string[]} published                       slugs written
 * @property {Array<{ name: string, reason: string }>} excluded
 */

/**
 * @param {string} base
 * @returns {string} base URL with any trailing slash removed
 */
function trimTrailingSlash(base) {
  return base.replace(/\/+$/, '');
}

/**
 * Fetch the full authority list (anonymous, no key). Throws on any non-OK
 * status or unexpected shape so a broken upstream fails the build loudly.
 *
 * @param {string} apiBase
 * @param {typeof globalThis.fetch} fetchImpl
 * @returns {Promise<Array<{ id: number, name: string, areaType: string }>>}
 */
async function fetchAuthorities(apiBase, fetchImpl) {
  const url = `${apiBase}/v1/authorities`;
  const res = await fetchImpl(url);
  if (!res.ok) {
    throw new Error(`GET /v1/authorities failed: HTTP ${res.status}`);
  }
  const body = await res.json();
  // The Go handler returns { authorities: [...], total }. Accept a bare array
  // defensively too.
  const list = Array.isArray(body) ? body : body?.authorities;
  if (!Array.isArray(list)) {
    throw new Error(
      'GET /v1/authorities returned an unexpected shape (no authorities array)',
    );
  }
  for (const a of list) {
    if (
      typeof a?.id !== 'number' ||
      typeof a?.name !== 'string' ||
      typeof a?.areaType !== 'string'
    ) {
      throw new Error('GET /v1/authorities returned a malformed authority row');
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
 * @param {string} outDir
 * @param {string} fixturePath
 * @param {number} limit
 * @param {{ warn: (msg: string) => void }} logger
 * @returns {Promise<PrerenderResult>}
 */
async function runFixtureMode(outDir, fixturePath, limit, logger) {
  const raw = await readFile(fixturePath, 'utf-8');
  const entries = JSON.parse(raw);
  if (!Array.isArray(entries)) {
    throw new Error(`fixture ${fixturePath} must be a JSON array`);
  }

  /** @type {string[]} */
  const published = [];
  /** @type {Array<{ name: string, reason: string }>} */
  const excluded = [];
  const seenSlugs = new Set();

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
      applications: Array.isArray(entry.applications) ? entry.applications : [],
      limit,
      published,
      excluded,
      seenSlugs,
      logger,
    });
  }

  await writeFile(join(outDir, 'sitemap.xml'), renderSitemap(published), 'utf-8');
  return { skipped: false, published, excluded };
}

/**
 * @param {Object} args
 * @param {string} args.outDir
 * @param {string} args.apiBase
 * @param {string} args.buildKey
 * @param {number} args.limit
 * @param {typeof globalThis.fetch} args.fetchImpl
 * @param {{ warn: (msg: string) => void }} args.logger
 * @returns {Promise<PrerenderResult>}
 */
async function runLiveMode(args) {
  const { outDir, apiBase, buildKey, limit, fetchImpl, logger } = args;

  const authorities = await fetchAuthorities(apiBase, fetchImpl);

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

  await writeFile(join(outDir, 'sitemap.xml'), renderSitemap(published), 'utf-8');
  return { skipped: false, published, excluded };
}

/**
 * Orchestrate the prerender. Dependency-injected for testing.
 *
 * @param {Object} options
 * @param {string} options.outDir                  directory to emit into (web/dist)
 * @param {string} [options.apiBase]               API base URL (live mode)
 * @param {string} [options.buildKey]              SITE_BUILD_KEY (live mode)
 * @param {string} [options.fixturePath]           committed JSON fixture (fixture mode)
 * @param {number} [options.limit]                 applications rendered per page
 * @param {typeof globalThis.fetch} [options.fetchImpl]
 * @param {{ log: Function, warn: Function, error: Function }} [options.logger]
 * @returns {Promise<PrerenderResult>}
 */
export async function runPrerender(options) {
  const {
    outDir,
    apiBase,
    buildKey,
    fixturePath,
    limit = DEFAULT_LIMIT,
    fetchImpl = globalThis.fetch,
    logger = console,
  } = options;

  if (fixturePath) {
    return runFixtureMode(outDir, fixturePath, limit, logger);
  }

  if (!buildKey) {
    return {
      skipped: true,
      reason: 'SITE_BUILD_KEY not set',
      published: [],
      excluded: [],
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
  const scriptDir = dirname(fileURLToPath(import.meta.url));
  const outDir = process.env.PRERENDER_OUT_DIR || join(scriptDir, '..', 'dist');
  const apiBase =
    process.env.PRERENDER_API_BASE || process.env.VITE_API_BASE_URL;
  const buildKey = process.env.SITE_BUILD_KEY;
  const fixturePath = process.env.PRERENDER_FIXTURE;

  try {
    const result = await runPrerender({
      outDir,
      apiBase,
      buildKey,
      fixturePath,
    });
    if (result.skipped) {
      console.log(`[prerender] skipped — ${result.reason} (SPA build only)`);
      return;
    }
    console.log(
      `[prerender] wrote ${result.published.length} planning page(s); ` +
        `excluded ${result.excluded.length} authority/-ies`,
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
