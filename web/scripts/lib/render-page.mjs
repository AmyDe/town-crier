import { SITE_ORIGIN, APPLE_APP_ID, appStoreUrl, shareUrl } from './constants.mjs';
import { escapeHtml, leadLine } from './format.mjs';
import { townPagePath } from './town-path.mjs';
import {
  pageStyles,
  renderApplicationsList,
  renderStatusSummary,
  renderDataUpdated,
  renderAttributionList,
} from './render-shared.mjs';

/**
 * @typedef {import('./render-shared.mjs').PlanningApplicationItem} PlanningApplicationItem
 */

/**
 * @typedef {Object} TownLink
 * @property {string} name display name of the published town
 * @property {string} slug the town's URL slug (joined under this authority)
 */

/**
 * @typedef {Object} PlanningPageData
 * @property {string} slug
 * @property {string} areaName
 * @property {number} authorityId
 * @property {number} total
 * @property {Array<{ appState: string | null, count: number }>} statusBreakdown
 * @property {PlanningApplicationItem[]} applications
 * @property {TownLink[]} [towns] published town children, for the internal link
 *   list. Omitted/empty for authorities with no published towns.
 */

/**
 * Render the internal link list down to this authority's published town pages.
 * Only published (coverage-gated) towns are passed in, so these links never
 * point at a 404. Returns '' when there are no towns, so the section is omitted
 * entirely. Town display names are HTML-escaped; slugs are already URL-safe.
 *
 * @param {PlanningPageData} data
 * @returns {string}
 */
function renderTownLinks(data) {
  if (!Array.isArray(data.towns) || data.towns.length === 0) {
    return '';
  }
  const area = escapeHtml(data.areaName);
  const items = data.towns
    .map(
      (town) =>
        `          <li><a href="/planning/${townPagePath(data.slug, town.slug)}">${escapeHtml(town.name)}</a></li>`,
    )
    .join('\n');
  return `
        <section class="townLinks">
          <h2>Planning applications by town in ${area}</h2>
          <p>Browse recent applications for individual towns and villages in ${area}.</p>
          <ul class="townLinks__list">
${items}
          </ul>
        </section>
`;
}

/**
 * @param {PlanningPageData} data
 * @param {string} canonical
 * @returns {string} JSON-LD, safe to embed inside a <script> element
 */
function buildJsonLd(data, canonical) {
  const itemList = {
    '@context': 'https://schema.org',
    '@type': 'ItemList',
    name: `Planning applications in ${data.areaName}`,
    url: canonical,
    numberOfItems: data.applications.length,
    itemListElement: data.applications.map((app, i) => ({
      '@type': 'ListItem',
      position: i + 1,
      name: [app.name, app.address].filter(Boolean).join(' — '),
      // Prefer our own share page (the application's canonical Town Crier page)
      // as the item URL; fall back to the council/PlanIt record, then the page.
      url: shareUrl(data.slug, app.name) || app.url || app.link || canonical,
    })),
  };
  const dataset = {
    '@context': 'https://schema.org',
    '@type': 'Dataset',
    name: `Recent planning applications in ${data.areaName}`,
    description: `Recent local planning applications in ${data.areaName}, drawn from Town Crier's planning-application snapshot.`,
    url: canonical,
    isAccessibleForFree: true,
    creator: {
      '@type': 'Organization',
      name: 'PlanIt',
      url: 'https://planit.org.uk',
    },
    license:
      'https://www.nationalarchives.gov.uk/doc/open-government-licence/version/3/',
  };
  // Escape "<" so a malicious data value can never close the <script> element.
  return JSON.stringify([itemList, dataset]).replace(/</g, '\\u003c');
}

/**
 * Render a complete, hydration-free static HTML page for one authority.
 *
 * @param {PlanningPageData} data
 * @returns {string}
 */
export function renderPlanningPage(data) {
  const area = escapeHtml(data.areaName);
  const canonical = `${SITE_ORIGIN}/planning/${data.slug}`;
  const lead = escapeHtml(leadLine(data.areaName, data.total));
  const title = `Planning applications in ${area} | Town Crier`;
  const metaDescription = escapeHtml(
    `Recent planning applications in ${data.areaName}. See what is being built nearby and get push alerts the moment an application is submitted or decided in your area.`,
  );
  const jsonLd = buildJsonLd(data, canonical);
  const year = new Date().getFullYear();

  const applicationsList = renderApplicationsList(data.applications, data.slug);
  const townLinks = renderTownLinks(data);
  const attribution = renderAttributionList();
  const dataUpdated = renderDataUpdated(data.applications);

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
    <meta property="og:title" content="Planning applications in ${area}" />
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
        <a href="/">Town Crier</a>
        <nav class="siteHeader__nav">
          <a href="/">Home</a>
          <a class="siteHeader__cta" href="${appStoreUrl('seo-lpa')}" rel="noopener" target="_blank">Get the app</a>
        </nav>
      </header>
      <main>
        <h1>Planning applications in ${area}</h1>
        ${dataUpdated}
        <p class="lead">${lead}</p>

${renderStatusSummary(data.statusBreakdown)}

        <h2>Recent applications</h2>
        <ul class="appList">
${applicationsList}
        </ul>
${townLinks}
        <section class="explainer">
          <h2>How to comment on a planning application in ${area}</h2>
          <p>
            Anyone can have their say on a planning application in ${area}. Find the
            application on the ${area} planning authority's public-access portal using
            the reference number above, then submit a comment before the consultation
            deadline. Comments are usually limited to planning matters such as overlooking,
            loss of light, traffic, noise and the character of the area.
          </p>
          <p>
            ${area} is the local planning authority responsible for deciding these
            applications. Town Crier mirrors the public record so you can keep track of
            what is happening near you, but the council remains the authoritative source
            and the place to submit formal comments.
          </p>
        </section>

        <section class="cta">
          <h2>Get push alerts for ${area}</h2>
          <p>
            Draw a circle on the map and Town Crier will notify you the moment a new
            planning application is submitted or decided inside it.
          </p>
          <a class="cta__button" href="${appStoreUrl('seo-lpa')}" rel="noopener" target="_blank">Download on the App Store</a>
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
