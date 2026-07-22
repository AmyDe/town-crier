/**
 * Static renderer for the `/planning/` authority hub page — a single A-Z index
 * of every published authority page (tc-geq7h.1, GH #821 Phase 1). Same
 * hydration-free template family as the authority (`render-page.mjs`) and town
 * (`render-town-page.mjs`) pages, reusing the shared inline-CTA/attribution/
 * stylesheet building blocks from `render-shared.mjs`.
 *
 * Rendered by `prerender-planning.mjs --render` from the existing
 * `seo-snapshot.json` — this page adds ZERO new API calls. Every authority it
 * lists already published its own page in the SAME render pass (the caller
 * only ever hands in published entries), so a hub link can never point at a
 * 404. Grouped strictly A-Z, never by region: authorities carry no
 * region/nation data anywhere in the snapshot or `internal/authorities`, and
 * sourcing one is explicitly out of scope (GH #821).
 */

import { SITE_ORIGIN, APPLE_APP_ID, appStoreUrl } from './constants.mjs';
import { escapeHtml } from './format.mjs';
import {
  pageStyles,
  renderInlineCta,
  renderAttributionList,
  renderPlanningCrossLinks,
} from './render-shared.mjs';

/**
 * @typedef {Object} PlanningIndexEntry
 * @property {string} name             authority display name
 * @property {string} slug             URL slug, e.g. "basingstoke-and-deane"
 * @property {number} applicationCount recent-application total — the same
 *   bounded count shown on the authority's own page
 * @property {number} townCount        published town-page count under this
 *   authority (0 when it has none)
 */

/**
 * @typedef {Object} PlanningIndexData
 * @property {PlanningIndexEntry[]} authorities  every PUBLISHED authority, in
 *   the exact order to render. The caller sorts alphabetically upstream (this
 *   module groups by first letter but does not itself re-sort) — mirroring how
 *   `render-page.mjs`'s town-links section renders whatever order it's given.
 */

/** App Store Connect campaign token for every CTA on this page. */
const CAMPAIGN = 'seo-hub';

/**
 * Group already-sorted authority entries into per-letter buckets, keyed by the
 * uppercased first character of the display name. Preserves input order
 * within each bucket, and returns buckets in first-seen order — since the
 * caller supplies an alphabetically-sorted array, first-seen order IS A-Z
 * order. A letter with zero entries never produces a bucket at all.
 *
 * @param {ReadonlyArray<PlanningIndexEntry>} authorities
 * @returns {Array<{ letter: string, entries: PlanningIndexEntry[] }>}
 */
function groupByLetter(authorities) {
  /** @type {Map<string, PlanningIndexEntry[]>} */
  const groups = new Map();
  for (const authority of authorities) {
    const trimmed = authority.name.trim();
    const letter = (trimmed.length > 0 ? trimmed[0] : '#').toUpperCase();
    const bucket = groups.get(letter);
    if (bucket) {
      bucket.push(authority);
    } else {
      groups.set(letter, [authority]);
    }
  }
  return [...groups.entries()].map(([letter, entries]) => ({ letter, entries }));
}

/**
 * Build the "N applications tracked [· N towns]" metadata line for one
 * authority. The town-count clause is entirely omitted when the authority has
 * zero published town pages, rather than reading as "0 towns".
 *
 * @param {PlanningIndexEntry} authority
 * @returns {string} plain text (caller HTML-escapes)
 */
function metaLine(authority) {
  const applicationsWord = authority.applicationCount === 1 ? 'application' : 'applications';
  const parts = [`${authority.applicationCount} ${applicationsWord} tracked`];
  if (authority.townCount > 0) {
    const townsWord = authority.townCount === 1 ? 'town' : 'towns';
    parts.push(`${authority.townCount} ${townsWord}`);
  }
  return parts.join(' · ');
}

/**
 * @param {PlanningIndexEntry} authority
 * @returns {string}
 */
function renderAuthorityEntry(authority) {
  const name = escapeHtml(authority.name);
  const meta = escapeHtml(metaLine(authority));
  return `            <li class="hubList__item">
              <a class="hubList__link" href="/planning/${authority.slug}">${name}</a>
              <span class="hubList__meta">${meta}</span>
            </li>`;
}

/**
 * @param {ReadonlyArray<{ letter: string, entries: PlanningIndexEntry[] }>} groups
 * @returns {string} '' when there are no groups at all
 */
function renderLetterNav(groups) {
  if (groups.length === 0) {
    return '';
  }
  const links = groups
    .map(({ letter }) => `<a href="#letter-${letter}">${letter}</a>`)
    .join('\n          ');
  return `
        <nav class="azNav" aria-label="Jump to letter">
          ${links}
        </nav>`;
}

/**
 * @param {ReadonlyArray<{ letter: string, entries: PlanningIndexEntry[] }>} groups
 * @returns {string}
 */
function renderLetterSections(groups) {
  return groups
    .map(({ letter, entries }) => {
      const items = entries.map(renderAuthorityEntry).join('\n');
      return `        <section class="hubGroup" id="letter-${letter}" aria-labelledby="letter-${letter}-heading">
          <h2 id="letter-${letter}-heading">${letter}</h2>
          <ul class="hubList">
${items}
          </ul>
        </section>`;
    })
    .join('\n');
}

/**
 * @param {PlanningIndexData} data
 * @param {string} canonical
 * @returns {string} JSON-LD, safe to embed inside a <script> element
 */
function buildJsonLd(data, canonical) {
  const itemList = {
    '@context': 'https://schema.org',
    '@type': 'ItemList',
    name: 'UK local planning authorities',
    url: canonical,
    numberOfItems: data.authorities.length,
    itemListElement: data.authorities.map((authority, i) => ({
      '@type': 'ListItem',
      position: i + 1,
      name: authority.name,
      url: `${SITE_ORIGIN}/planning/${authority.slug}`,
    })),
  };
  // The hub has no parent above it (it IS the top of the /planning tree), so
  // this trail is two levels: Home -> this page — matching the authority
  // page's own two-level Home -> Authority trail.
  const breadcrumb = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: [
      { '@type': 'ListItem', position: 1, name: 'Town Crier', item: `${SITE_ORIGIN}/` },
      {
        '@type': 'ListItem',
        position: 2,
        name: 'Planning applications',
        item: canonical,
      },
    ],
  };
  // Escape "<" so a malicious data value can never close the <script> element.
  return JSON.stringify([itemList, breadcrumb]).replace(/</g, '\\u003c');
}

/**
 * Render the complete, hydration-free static HTML `/planning/` hub page: an
 * A-Z index of every published authority page.
 *
 * @param {PlanningIndexData} data
 * @returns {string}
 */
export function renderPlanningIndexPage(data) {
  const canonical = `${SITE_ORIGIN}/planning`;
  const count = data.authorities.length;
  const authoritiesNoun = count === 1 ? 'local planning authority' : 'local planning authorities';
  const title = 'Planning applications by council | Town Crier';
  const metaDescription = escapeHtml(
    `Browse recent planning applications for ${count} ${authoritiesNoun} across the UK. Find your council and get push alerts the moment a new application is submitted or decided.`,
  );
  const jsonLd = buildJsonLd(data, canonical);
  const year = new Date().getFullYear();
  const groups = groupByLetter(data.authorities);
  const letterNav = renderLetterNav(groups);
  const letterSections = renderLetterSections(groups);
  const attribution = renderAttributionList();
  const storeHref = appStoreUrl(CAMPAIGN);

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
    <meta property="og:title" content="Planning applications by council" />
    <meta property="og:description" content="${metaDescription}" />
    <meta property="og:type" content="website" />
    <meta property="og:url" content="${canonical}" />
    <meta property="og:site_name" content="Town Crier" />
    <script type="application/ld+json">${jsonLd}</script>
    <style>
${pageStyles()}
    </style>
  </head>
  <body>
    <div class="wrap">
      <header class="siteHeader">
        <div class="siteHeader__inner">
          <a href="/" class="siteHeader__wordmark">Town Crier</a>
          <a class="siteHeader__cta" href="${storeHref}" rel="noopener" target="_blank">Get the app</a>
        </div>
        <div class="siteHeader__ruleHeavy"></div>
        <div class="siteHeader__ruleHairline"></div>
      </header>
      <nav class="breadcrumb" aria-label="Breadcrumb">
        <ol>
          <li><a href="/">Town Crier</a></li>
          <li>Planning applications</li>
        </ol>
      </nav>
      <main>
        <h1>Planning applications by council</h1>
        <p class="lead">Browse recent planning applications for ${count} ${authoritiesNoun} across the UK.</p>
${renderInlineCta('your council', storeHref)}
${renderPlanningCrossLinks()}
${letterNav}
${letterSections}

        <section class="cta">
          <h2 class="cta__heading">Get push alerts for your area</h2>
          <p>
            Draw a circle on the map and Town Crier will notify you the moment a new
            planning application is submitted or decided inside it.
          </p>
          <a class="cta__button" href="${storeHref}" rel="noopener" target="_blank">Download on the App Store</a>
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
