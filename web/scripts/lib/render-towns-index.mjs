/**
 * Static renderer for the town INDEX page (`/planning/towns/`): a single,
 * unpaginated A-Z directory of every published town page (~1,550), each
 * entry linking to `/planning/<authority-slug>/<town-slug>` with its parent
 * authority name shown as visible context. Same hydration-free static-HTML
 * template family as `render-page.mjs` / `render-town-page.mjs` — rendered
 * from data the town pass in `prerender-planning.mjs` already gathered, so
 * this page needs zero new API calls (GH #821 Phase 2).
 *
 * Town-page path resolution — including the same-name-as-authority 301
 * suppression (tc-77ll / #717) — happens upstream in `considerTown`
 * (`prerender-planning.mjs`) via `town-path.mjs`. This renderer only ever
 * sees towns that already cleared every gate, so it never links to a 404.
 */

import {
  SITE_ORIGIN,
  APPLE_APP_ID,
  appStoreUrl,
  TOWN_ATTRIBUTION_LINES,
} from './constants.mjs';
import { escapeHtml } from './format.mjs';
import { slugify } from './slug.mjs';
import { townPagePath } from './town-path.mjs';
import { pageStyles, renderInlineCta, renderAttributionList } from './render-shared.mjs';

/**
 * The reserved path segment this page owns (`/planning/towns`). Exported so
 * `prerender-planning.mjs` can guard against a real authority ever slugifying
 * to the same value — see {@link assertNoTownsSlugCollision}.
 * @type {string}
 */
export const TOWNS_INDEX_SLUG = 'towns';

/**
 * @typedef {Object} TownIndexEntry
 * @property {string} townName       display name, e.g. "Truro"
 * @property {string} townSlug       lowercase-hyphenated, e.g. "truro"
 * @property {string} authoritySlug  parent authority slug, e.g. "cornwall"
 * @property {string} authorityName  parent authority display name
 */

/**
 * Fail loudly if any authority in the FULL committed/snapshot authority list
 * would slugify to the same path segment this index page owns
 * (`/planning/towns`) — that authority's own page would collide with
 * `dist/planning/towns/index.html`. Checked against every authority passed
 * in, not just the ones that end up publishing a page, so an authority that
 * is excluded today (e.g. by areaType) can never silently start colliding
 * after a future data change goes unnoticed.
 *
 * @param {ReadonlyArray<{ name: string }>} authorities
 * @returns {void}
 */
export function assertNoTownsSlugCollision(authorities) {
  const colliding = authorities.find((a) => slugify(a.name) === TOWNS_INDEX_SLUG);
  if (colliding) {
    throw new Error(
      `authority "${colliding.name}" slugifies to "${TOWNS_INDEX_SLUG}", which ` +
        `collides with the town index route (/planning/${TOWNS_INDEX_SLUG}) — ` +
        `rename or special-case it before rendering`,
    );
  }
}

/**
 * The bucket label for a town name that doesn't start with an A-Z letter. No
 * town in the current gazetteer needs it, but it keeps grouping total —
 * every entry lands in a section, never silently dropped.
 * @type {string}
 */
const OTHER_LETTER = '#';

/**
 * @param {string} name
 * @returns {string} the single uppercase A-Z bucket letter, or {@link OTHER_LETTER}
 */
function indexLetter(name) {
  const first = name.trim().charAt(0).toUpperCase();
  return first >= 'A' && first <= 'Z' ? first : OTHER_LETTER;
}

/**
 * @typedef {Object} TownIndexSection
 * @property {string} letter
 * @property {TownIndexEntry[]} entries
 */

/**
 * Group town entries into A-Z sections, each internally sorted by town name
 * then — for towns sharing a name across different authorities, e.g. two
 * "Richmond"s — by authority name, so the ordering is fully deterministic.
 * Sections are ordered A-Z with any {@link OTHER_LETTER} bucket last.
 *
 * @param {ReadonlyArray<TownIndexEntry>} entries
 * @returns {TownIndexSection[]}
 */
export function groupTownIndexEntries(entries) {
  /** @type {Map<string, TownIndexEntry[]>} */
  const buckets = new Map();
  for (const entry of entries) {
    const letter = indexLetter(entry.townName);
    const bucket = buckets.get(letter);
    if (bucket) {
      bucket.push(entry);
    } else {
      buckets.set(letter, [entry]);
    }
  }

  const letters = [...buckets.keys()].sort((a, b) => {
    if (a === OTHER_LETTER) return 1;
    if (b === OTHER_LETTER) return -1;
    return a.localeCompare(b);
  });

  return letters.map((letter) => {
    const bucket = buckets.get(letter);
    return {
      letter,
      entries: [...bucket].sort(
        (a, b) =>
          a.townName.localeCompare(b.townName) ||
          a.authorityName.localeCompare(b.authorityName),
      ),
    };
  });
}

/**
 * @param {TownIndexSection[]} sections
 * @returns {string}
 */
function renderJumpNav(sections) {
  const links = sections
    .map((s) => `<a href="#letter-${s.letter}">${s.letter}</a>`)
    .join('\n          ');
  return `        <nav class="townsIndex__jump" aria-label="Jump to letter">
          ${links}
        </nav>`;
}

/**
 * @param {TownIndexSection} section
 * @returns {string}
 */
function renderSection(section) {
  const items = section.entries
    .map((entry) => {
      const path = townPagePath(entry.authoritySlug, entry.townSlug);
      return `            <li class="townsIndex__item"><a href="/planning/${path}">${escapeHtml(entry.townName)}</a> <span class="townsIndex__authority">${escapeHtml(entry.authorityName)}</span></li>`;
    })
    .join('\n');
  return `        <section class="townsIndex__section" id="letter-${section.letter}" aria-labelledby="letter-${section.letter}-heading">
          <h2 id="letter-${section.letter}-heading">${section.letter}</h2>
          <ul class="townsIndex__list">
${items}
          </ul>
        </section>`;
}

/**
 * @param {string} canonical
 * @returns {string} JSON-LD, safe to embed inside a <script> element
 */
function buildJsonLd(canonical) {
  const breadcrumb = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: [
      { '@type': 'ListItem', position: 1, name: 'Town Crier', item: `${SITE_ORIGIN}/` },
      { '@type': 'ListItem', position: 2, name: 'Towns', item: canonical },
    ],
  };
  // Escape "<" so a malicious data value can never close the <script> element.
  return JSON.stringify([breadcrumb]).replace(/</g, '\\u003c');
}

/**
 * Additional styles for the A-Z town index layout. Kept local to this
 * template (rather than folded into the shared `pageStyles()`) so this Phase
 * 2 page and the Phase 1 authority-index page (built concurrently, tc-geq7h.1)
 * can each extend the shared stylesheet independently without both editing
 * the same lines of `render-shared.mjs`. References the SAME `var(--tc-*)`
 * tokens `pageStyles()` already defines in its `:root` block.
 * @returns {string}
 */
function townsIndexStyles() {
  return `
    .townsIndex__jump { display: flex; flex-wrap: wrap; gap: var(--tc-space-sm); margin: 0 0 var(--tc-space-lg); font-weight: 600; }
    .townsIndex__jump a { color: var(--tc-amber); text-decoration: none; }
    .townsIndex__jump a:hover { text-decoration: underline; }
    .townsIndex__section { margin: var(--tc-space-lg) 0; }
    .townsIndex__section h2 { border-bottom: 1px solid var(--tc-border); padding-bottom: var(--tc-space-sm); }
    .townsIndex__list { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--tc-space-sm); }
    .townsIndex__item a { color: var(--tc-text-primary); font-weight: 600; text-decoration: none; }
    .townsIndex__item a:hover { color: var(--tc-amber); text-decoration: underline; }
    .townsIndex__authority { font-family: var(--tc-font-mono); color: var(--tc-text-secondary); font-size: 0.875rem; }`;
}

/**
 * Render a complete, hydration-free static HTML page listing every published
 * town page A-Z. Single page for v1 — GH #821 Phase 2 explicitly defers
 * pagination or per-letter splitting until it proves a real problem.
 *
 * @param {ReadonlyArray<TownIndexEntry>} entries
 * @returns {string}
 */
export function renderTownsIndexPage(entries) {
  const canonical = `${SITE_ORIGIN}/planning/${TOWNS_INDEX_SLUG}`;
  const title = 'Planning applications by town | Town Crier';
  const metaDescription = escapeHtml(
    'Browse recent planning applications by town across England, Wales and Scotland. Pick a town to see what is being built nearby, or get push alerts the moment something changes.',
  );
  const sections = groupTownIndexEntries(entries);
  const jsonLd = buildJsonLd(canonical);
  const year = new Date().getFullYear();
  const attribution = renderAttributionList(TOWN_ATTRIBUTION_LINES);
  const noun = entries.length === 1 ? 'town' : 'towns';
  const cta = appStoreUrl('seo-towns-index');

  const body =
    sections.length > 0
      ? `${renderJumpNav(sections)}

${sections.map(renderSection).join('\n')}`
      : `        <p class="townsIndex__empty">No towns are published yet — check back soon, or browse by <a href="/planning/">local authority</a> instead.</p>`;

  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="apple-itunes-app" content="app-id=${APPLE_APP_ID}" />
    <title>${title}</title>
    <meta name="description" content="${metaDescription}" />
    <meta name="robots" content="index,follow" />
    <link rel="canonical" href="${canonical}" />
    <link rel="icon" type="image/x-icon" href="/favicon.ico" />
    <meta property="og:title" content="Planning applications by town" />
    <meta property="og:description" content="${metaDescription}" />
    <meta property="og:type" content="website" />
    <meta property="og:url" content="${canonical}" />
    <meta property="og:site_name" content="Town Crier" />
    <script type="application/ld+json">${jsonLd}</script>
    <style>
${pageStyles()}
${townsIndexStyles()}
    </style>
  </head>
  <body>
    <div class="wrap">
      <header class="siteHeader">
        <div class="siteHeader__inner">
          <a href="/" class="siteHeader__wordmark">Town Crier</a>
          <a class="siteHeader__cta" href="${cta}" rel="noopener" target="_blank">Get the app</a>
        </div>
        <div class="siteHeader__ruleHeavy"></div>
        <div class="siteHeader__ruleHairline"></div>
      </header>
      <nav class="breadcrumb" aria-label="Breadcrumb">
        <ol>
          <li><a href="/">Town Crier</a></li>
          <li>Towns</li>
        </ol>
      </nav>
      <main>
        <h1>Planning applications by town</h1>
        <p class="lead">Browse ${entries.length} ${noun} we track, A to Z. Pick yours to see what's being built nearby.</p>
${renderInlineCta('your town', cta)}

${body}

        <section class="cta">
          <h2 class="cta__heading">Get push alerts for your town</h2>
          <p>
            Draw a circle on the map and Town Crier will notify you the moment a new
            planning application is submitted or decided inside it.
          </p>
          <a class="cta__button" href="${cta}" rel="noopener" target="_blank">Download on the App Store</a>
        </section>
      </main>

      <footer class="siteFooter">
        <ul class="attribution">
${attribution}
        </ul>
        <p>© ${year} Town Crier · Ivo and the Bea Ltd</p>
        <p><a href="/legal/privacy">Privacy Policy</a> · <a href="/legal/terms">Terms of Service</a></p>
      </footer>
    </div>
  </body>
</html>
`;
}
