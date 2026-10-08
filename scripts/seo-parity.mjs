#!/usr/bin/env node
// Compares SEO head fields between the live site and the preview site page by page.
// Usage: node scripts/seo-parity.mjs [--live <origin>] [--preview <origin>]
// Exits 0 when the URL sets and every compared field match, 1 on any difference, 2 on bad usage.

import { pathToFileURL } from 'node:url';

export const DEFAULT_LIVE = 'https://towncrierapp.uk';
export const DEFAULT_PREVIEW = 'https://preview.towncrierapp.uk';
const USER_AGENT = 'town-crier-seo-parity/1.0 (+https://towncrierapp.uk)';
const REQUEST_TIMEOUT_MS = 30_000;

export const FIELDS = ['status', 'title', 'description', 'canonical', 'robots', 'ogTitle', 'ogUrl', 'h1', 'jsonLdTypes'];

export function parseArgs(argv) {
  const opts = { live: DEFAULT_LIVE, preview: DEFAULT_PREVIEW };
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg !== '--live' && arg !== '--preview') {
      throw new Error(`unknown argument: ${arg}`);
    }
    const value = argv[++i];
    if (!value) throw new Error(`${arg} needs a value`);
    opts[arg.slice(2)] = new URL(value).origin;
  }
  return opts;
}

const NAMED_ENTITIES = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ' };

export function decodeEntities(text) {
  return text.replace(/&(#x[0-9a-f]+|#[0-9]+|[a-z]+);/gi, (match, entity) => {
    if (entity[0] === '#') {
      const code = entity[1].toLowerCase() === 'x' ? parseInt(entity.slice(2), 16) : parseInt(entity.slice(1), 10);
      return Number.isFinite(code) ? String.fromCodePoint(code) : match;
    }
    return NAMED_ENTITIES[entity.toLowerCase()] ?? match;
  });
}

function normaliseText(text) {
  return decodeEntities(text.replace(/<[^>]*>/g, '')).replace(/\s+/g, ' ').trim();
}

export function parseAttributes(tag) {
  const attrs = {};
  const body = tag.replace(/^<\s*[a-z0-9-]+/i, '').replace(/\/?>$/, '');
  const re = /([^\s=/>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?/g;
  let m;
  while ((m = re.exec(body)) !== null) {
    attrs[m[1].toLowerCase()] = decodeEntities(m[2] ?? m[3] ?? m[4] ?? '');
  }
  return attrs;
}

function tags(html, name) {
  return [...html.matchAll(new RegExp(`<${name}\\b[^>]*>`, 'gi'))].map((m) => parseAttributes(m[0]));
}

function metaContent(metas, key, value) {
  const found = metas.find((attrs) => (attrs[key] ?? '').toLowerCase() === value);
  return found ? found.content ?? '' : null;
}

export function jsonLdTypes(html) {
  const types = [];
  const scripts = html.matchAll(/<script\b[^>]*type\s*=\s*["']?application\/ld\+json["']?[^>]*>([\s\S]*?)<\/script>/gi);
  for (const [, json] of scripts) {
    let data;
    try {
      data = JSON.parse(json);
    } catch {
      types.push('<invalid JSON-LD>');
      continue;
    }
    const nodes = Array.isArray(data) ? data : [data];
    for (const node of nodes) {
      const members = Array.isArray(node?.['@graph']) ? node['@graph'] : [node];
      for (const member of members) {
        const type = member?.['@type'];
        if (Array.isArray(type)) types.push(...type.map(String));
        else if (type !== undefined) types.push(String(type));
      }
    }
  }
  return types.sort();
}

/** Extracts the compared SEO fields from an HTML document. Missing fields are null. */
export function extractSeoFields(html) {
  const title = html.match(/<title\b[^>]*>([\s\S]*?)<\/title>/i);
  const h1 = html.match(/<h1\b[^>]*>([\s\S]*?)<\/h1>/i);
  const metas = tags(html, 'meta');
  const canonical = tags(html, 'link').find((attrs) => (attrs.rel ?? '').toLowerCase().split(/\s+/).includes('canonical'));
  return {
    title: title ? normaliseText(title[1]) : null,
    description: metaContent(metas, 'name', 'description'),
    canonical: canonical ? canonical.href ?? '' : null,
    robots: metaContent(metas, 'name', 'robots'),
    ogTitle: metaContent(metas, 'property', 'og:title'),
    ogUrl: metaContent(metas, 'property', 'og:url'),
    h1: h1 ? normaliseText(h1[1]) : null,
    jsonLdTypes: jsonLdTypes(html).join(', '),
  };
}

/** Returns the URL paths listed in a sitemap, in document order. */
export function sitemapPaths(xml) {
  return [...xml.matchAll(/<loc>\s*([\s\S]*?)\s*<\/loc>/gi)].map(([, loc]) => new URL(decodeEntities(loc)).pathname);
}

export function diffPathSets(livePaths, previewPaths) {
  const live = new Set(livePaths);
  const preview = new Set(previewPaths);
  return {
    onlyLive: [...live].filter((p) => !preview.has(p)),
    onlyPreview: [...preview].filter((p) => !live.has(p)),
    common: [...live].filter((p) => preview.has(p)),
  };
}

/** Lists fields whose values differ between two `extractSeoFields` results (plus `status`). */
export function compareFields(live, preview) {
  return FIELDS.filter((field) => live[field] !== preview[field]).map((field) => ({
    field,
    live: live[field],
    preview: preview[field],
  }));
}

async function get(url) {
  const response = await fetch(url, {
    headers: { 'User-Agent': USER_AGENT, Accept: 'text/html,application/xml' },
    redirect: 'manual',
    signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
  });
  return { status: response.status, body: await response.text() };
}

async function fetchPage(url) {
  try {
    const { status, body } = await get(url);
    return { status, ...extractSeoFields(body) };
  } catch (err) {
    return { status: `error: ${err.message}` };
  }
}

async function fetchSitemap(origin) {
  const { status, body } = await get(`${origin}/sitemap.xml`);
  if (status !== 200) throw new Error(`${origin}/sitemap.xml returned ${status}`);
  return sitemapPaths(body);
}

async function main() {
  let opts;
  try {
    opts = parseArgs(process.argv.slice(2));
  } catch (err) {
    console.error(err.message);
    console.error('usage: node scripts/seo-parity.mjs [--live <origin>] [--preview <origin>]');
    process.exit(2);
  }

  console.log(`live:    ${opts.live}\npreview: ${opts.preview}`);
  const livePaths = await fetchSitemap(opts.live);
  const previewPaths = await fetchSitemap(opts.preview);
  const sets = diffPathSets(livePaths, previewPaths);
  console.log(`sitemap: live ${livePaths.length} URLs, preview ${previewPaths.length} URLs`);
  for (const path of sets.onlyLive) console.log(`ONLY LIVE     ${path}`);
  for (const path of sets.onlyPreview) console.log(`ONLY PREVIEW  ${path}`);

  let differingPages = 0;
  for (const [index, path] of sets.common.entries()) {
    const live = await fetchPage(opts.live + path);
    const preview = await fetchPage(opts.preview + path);
    const diffs = compareFields(live, preview);
    if (diffs.length > 0) {
      differingPages++;
      console.log(`DIFF  ${path}`);
      for (const d of diffs) {
        console.log(`  ${d.field}\n    live:    ${JSON.stringify(d.live)}\n    preview: ${JSON.stringify(d.preview)}`);
      }
    }
    if ((index + 1) % 100 === 0) console.error(`checked ${index + 1}/${sets.common.length}`);
  }

  console.log(
    `\nsummary: ${sets.common.length} pages compared, ${differingPages} differ, ` +
      `${sets.onlyLive.length} only on live, ${sets.onlyPreview.length} only on preview`,
  );
  const ok = differingPages === 0 && sets.onlyLive.length === 0 && sets.onlyPreview.length === 0;
  console.log(ok ? 'PARITY OK' : 'PARITY FAILED');
  process.exit(ok ? 0 : 1);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((err) => {
    console.error(err);
    process.exit(2);
  });
}
