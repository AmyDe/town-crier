/**
 * Static renderer for a town-level SEO page (`/planning/<authority>/<town>`).
 * It is the same hydration-free template as the authority page (shared cards,
 * stats, attribution and stylesheet from `render-shared.mjs`), retitled to the
 * town and nested one level deeper:
 *
 *   - H1 + lead + CTA + explainer name the TOWN
 *   - a breadcrumb climbs back up to the parent authority page
 *   - the canonical / OG url / sitemap entry are the nested town URL
 *   - schema.org carries an ItemList, a Dataset and a BreadcrumbList
 *
 * Data comes only from our own geo endpoint (Cosmos-backed) — never PlanIt. The
 * build key never reaches this output.
 */

import {
  SITE_ORIGIN,
  APPLE_APP_ID,
  appStoreUrl,
  shareUrl,
  TOWN_ATTRIBUTION_LINES,
} from './constants.mjs';
import { escapeHtml, leadLine } from './format.mjs';
import {
  pageStyles,
  renderApplicationsList,
  renderStatusSummary,
  renderDataUpdated,
  renderInlineCta,
  renderQrBlock,
  renderAttributionList,
} from './render-shared.mjs';

/**
 * @typedef {Object} TownPageData
 * @property {string} townName        display name, e.g. "Truro"
 * @property {string} townSlug        lowercase-hyphenated, e.g. "truro"
 * @property {string} authorityName   parent authority display name
 * @property {string} authoritySlug   parent authority slug, e.g. "cornwall"
 * @property {number} authorityId
 * @property {number} total
 * @property {Array<{ appState: string | null, count: number }>} statusBreakdown
 * @property {import('./render-shared.mjs').PlanningApplicationItem[]} applications
 */

/**
 * Build the schema.org JSON-LD for a town page: an ItemList of the rendered
 * applications, a Dataset describing the page, and a BreadcrumbList that mirrors
 * the visible Home › Authority › Town trail. Safe to embed inside a <script>.
 *
 * @param {TownPageData} data
 * @param {string} canonical            absolute town-page URL
 * @param {string} authorityCanonical   absolute parent authority-page URL
 * @returns {string}
 */
function buildTownJsonLd(data, canonical, authorityCanonical) {
  const itemList = {
    '@context': 'https://schema.org',
    '@type': 'ItemList',
    name: `Planning applications in ${data.townName}`,
    url: canonical,
    numberOfItems: data.applications.length,
    itemListElement: data.applications.map((app, i) => ({
      '@type': 'ListItem',
      position: i + 1,
      name: [app.name, app.address].filter(Boolean).join(' — '),
      // Prefer our own share page (the application's canonical Town Crier page)
      // as the item URL; fall back to the council/PlanIt record, then the page.
      url: shareUrl(data.authoritySlug, app.name) || app.url || app.link || canonical,
    })),
  };
  const dataset = {
    '@context': 'https://schema.org',
    '@type': 'Dataset',
    name: `Recent planning applications in ${data.townName}`,
    description: `Recent local planning applications in and around ${data.townName}, ${data.authorityName}, drawn from Town Crier's planning-application snapshot.`,
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
  // Mirrors the visible four-level trail (tc-3ht16): Home -> the /planning hub
  // -> the parent authority -> this town. "Planning applications" matches the
  // hub's own self-referential label (render-planning-index.mjs's buildJsonLd)
  // and the authority page's breadcrumb (render-page.mjs's buildJsonLd) exactly.
  const breadcrumb = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: [
      { '@type': 'ListItem', position: 1, name: 'Town Crier', item: `${SITE_ORIGIN}/` },
      {
        '@type': 'ListItem',
        position: 2,
        name: 'Planning applications',
        item: `${SITE_ORIGIN}/planning`,
      },
      {
        '@type': 'ListItem',
        position: 3,
        name: data.authorityName,
        item: authorityCanonical,
      },
      { '@type': 'ListItem', position: 4, name: data.townName, item: canonical },
    ],
  };
  // Escape "<" so a malicious data value can never close the <script> element.
  return JSON.stringify([itemList, dataset, breadcrumb]).replace(/</g, '\\u003c');
}

/**
 * Render a complete, hydration-free static HTML page for one town.
 *
 * @param {TownPageData} data
 * @returns {string}
 */
export function renderTownPage(data) {
  const town = escapeHtml(data.townName);
  const authority = escapeHtml(data.authorityName);
  const canonical = `${SITE_ORIGIN}/planning/${data.authoritySlug}/${data.townSlug}`;
  const authorityCanonical = `${SITE_ORIGIN}/planning/${data.authoritySlug}`;
  // Site-relative path for the VISIBLE breadcrumb link only — every sibling
  // crumb (Home, the /planning hub) is relative, so this one must be too, or
  // it jumps off-host on any non-prod origin (local preview, dev, staging).
  // `authorityCanonical` stays absolute for the JSON-LD BreadcrumbList `item`,
  // which schema.org expects as a full url.
  const authorityPath = `/planning/${data.authoritySlug}`;
  const lead = escapeHtml(leadLine(data.townName, data.total));
  const title = `Planning applications in ${town} | Town Crier`;
  const metaDescription = escapeHtml(
    `Recent planning applications in ${data.townName}, ${data.authorityName}. See what is being built nearby and get push alerts the moment an application is submitted or decided near you.`,
  );
  const jsonLd = buildTownJsonLd(data, canonical, authorityCanonical);
  const year = new Date().getFullYear();

  // One ct token per CTA surface (all under the same pt provider token), so
  // App Analytics shows which placement actually sends people to the store.
  const applicationsList = renderApplicationsList(data.applications, data.authoritySlug, {
    area: data.townName,
    storeHref: appStoreUrl('seo-town-mid'),
  });
  // Town pages credit the ONS Built-Up-Area + NRS gazetteers (their centroid
  // sources) on top of the base PlanIt/OGL/OS/OSM lines; authority pages keep the
  // base list since they don't use the gazetteer.
  const attribution = renderAttributionList(TOWN_ATTRIBUTION_LINES);
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
    <meta property="og:title" content="Planning applications in ${town}" />
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
          <a class="siteHeader__cta" href="${appStoreUrl('seo-town-hdr')}" rel="noopener" target="_blank">Get the app</a>
        </div>
        <div class="siteHeader__ruleHeavy"></div>
        <div class="siteHeader__ruleHairline"></div>
      </header>
      <nav class="breadcrumb" aria-label="Breadcrumb">
        <ol>
          <li><a href="/">Town Crier</a></li>
          <li><a href="/planning">Planning applications</a></li>
          <li><a href="${authorityPath}">${authority}</a></li>
          <li>${town}</li>
        </ol>
      </nav>
      <main>
        <h1>Planning applications in ${town}</h1>
        ${dataUpdated}
        <p class="lead">${lead}</p>
${renderInlineCta(data.townName, appStoreUrl('seo-town-inline'))}

${renderStatusSummary(data.statusBreakdown)}

        <h2 class="ledger__heading">Latest notices</h2>
        <ul class="ledger">
${applicationsList}
        </ul>

        <section class="explainer">
          <h2>How to comment on a planning application in ${town}</h2>
          <p>
            Anyone can have their say on a planning application in ${town}. Find the
            application on the ${authority} planning authority's public-access portal
            using the reference number above, then submit a comment before the
            consultation deadline. Comments are usually limited to planning matters
            such as overlooking, loss of light, traffic, noise and the character of
            the area.
          </p>
          <p>
            ${authority} is the local planning authority responsible for deciding
            applications in ${town}. Town Crier mirrors the public record so you can
            keep track of what is happening near you, but the council remains the
            authoritative source and the place to submit formal comments.
          </p>
        </section>

        <section class="cta">
          <h2 class="cta__heading">Get push alerts for ${town}</h2>
          <p>
            Draw a circle on the map and Town Crier will notify you the moment a new
            planning application is submitted or decided inside it. Most councils
            allow around three weeks for comments, so hearing early matters.
          </p>
          <a class="cta__button" href="${appStoreUrl('seo-town-btm')}" rel="noopener" target="_blank">Download free on the App Store</a>
${renderQrBlock(appStoreUrl('seo-town-qr'))}
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
