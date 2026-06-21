import {
  SITE_ORIGIN,
  APP_DOWNLOAD_URL,
  ATTRIBUTION_LINES,
} from './constants.mjs';
import {
  escapeHtml,
  truncate,
  formatDate,
  statusDisplayLabel,
  countByState,
  leadLine,
} from './format.mjs';

/**
 * @typedef {Object} PlanningApplicationItem
 * @property {string} uid
 * @property {string} name
 * @property {string} address
 * @property {string} description
 * @property {string | null} appState
 * @property {string | null} startDate  yyyy-MM-dd
 * @property {string | null} link       council-portal deep link
 * @property {string | null} url        application detail link
 */

/**
 * @typedef {Object} PlanningPageData
 * @property {string} slug
 * @property {string} areaName
 * @property {number} authorityId
 * @property {number} total
 * @property {boolean} totalCapped
 * @property {PlanningApplicationItem[]} applications
 */

const MAX_DESCRIPTION_LENGTH = 160;

const STATUS_MODIFIER = {
  Permitted: 'permitted',
  Conditions: 'conditions',
  Rejected: 'rejected',
  Withdrawn: 'withdrawn',
  Appealed: 'appealed',
};

/**
 * @param {string | null} appState
 * @returns {string}
 */
function statusModifier(appState) {
  if (appState === null || appState === undefined) {
    return 'default';
  }
  return STATUS_MODIFIER[appState] ?? 'default';
}

/**
 * @param {PlanningApplicationItem} app
 * @returns {string}
 */
function renderApplication(app) {
  const label = escapeHtml(statusDisplayLabel(app.appState));
  const modifier = statusModifier(app.appState);
  const date = formatDate(app.startDate);
  const description = escapeHtml(truncate(app.description, MAX_DESCRIPTION_LENGTH));

  const links = [];
  if (app.link) {
    links.push(
      `<a class="appLink" href="${escapeHtml(app.link)}" rel="nofollow noopener" target="_blank">View on the council portal</a>`,
    );
  }
  if (app.url) {
    links.push(
      `<a class="appLink" href="${escapeHtml(app.url)}" rel="nofollow noopener" target="_blank">Application details</a>`,
    );
  }

  return `      <li class="appCard">
        <div class="appCard__head">
          <h3 class="appCard__ref">${escapeHtml(app.name)}</h3>
          <span class="status status--${modifier}">${label}</span>
        </div>
        <p class="appCard__address">${escapeHtml(app.address)}</p>
        <p class="appCard__desc">${description}</p>
        <div class="appCard__meta">
          ${date ? `<span class="appCard__date">${escapeHtml(date)}</span>` : ''}
          ${links.join('\n          ')}
        </div>
      </li>`;
}

/**
 * @param {PlanningApplicationItem[]} applications
 * @returns {string}
 */
function renderStats(applications) {
  const rows = countByState(applications)
    .map(
      (s) =>
        `        <li class="statRow"><span class="statRow__label">${escapeHtml(s.label)}</span><span class="statRow__count">${s.count}</span></li>`,
    )
    .join('\n');
  return `    <section class="stats" aria-label="Application status breakdown">
      <h2 class="stats__heading">Status breakdown</h2>
      <ul class="statList">
${rows}
      </ul>
    </section>`;
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
      url: app.url || app.link || canonical,
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
 * Inline stylesheet for the standalone static page. Self-contained (no external
 * CSS request) for first-byte readability and strong Core Web Vitals. Uses the
 * Town Crier design tokens — dark by default, light via prefers-color-scheme —
 * mirroring `src/styles/tokens.css`.
 *
 * @returns {string}
 */
function pageStyles() {
  return `    :root {
      --tc-amber: #E9A620;
      --tc-amber-hover: #F0B83A;
      --tc-background: #1A1A1E;
      --tc-surface: #242428;
      --tc-text-primary: #F1EFE9;
      --tc-text-secondary: #9B9590;
      --tc-text-on-accent: #1C1917;
      --tc-status-permitted: #34C759;
      --tc-status-conditions: #FF9500;
      --tc-status-rejected: #FF453A;
      --tc-status-withdrawn: #8E8A85;
      --tc-status-appealed: #A78BFA;
      --tc-status-default: #9B9590;
      --tc-border: #3A3A3F;
      --tc-radius-md: 12px;
      --tc-radius-full: 9999px;
      --tc-space-sm: 8px;
      --tc-space-md: 16px;
      --tc-space-lg: 24px;
      --tc-space-xl: 32px;
      --tc-space-xxl: 48px;
      --tc-font-family: 'Inter', system-ui, -apple-system, sans-serif;
      --tc-content-max-width: 760px;
    }
    @media (prefers-color-scheme: light) {
      :root {
        --tc-amber: #D4910A;
        --tc-amber-hover: #B87A08;
        --tc-background: #FAF8F5;
        --tc-surface: #FFFFFF;
        --tc-text-primary: #1C1917;
        --tc-text-secondary: #6B6560;
        --tc-text-on-accent: #FFFFFF;
        --tc-status-permitted: #1A7D37;
        --tc-status-conditions: #A85A0A;
        --tc-status-rejected: #C42B2B;
        --tc-status-withdrawn: #7A7570;
        --tc-status-appealed: #7C3AED;
        --tc-status-default: #6B6560;
        --tc-border: #E8E4DF;
      }
    }
    @font-face {
      font-family: 'Inter';
      font-style: normal;
      font-weight: 400 700;
      font-display: swap;
      src: url('/fonts/inter-latin.woff2') format('woff2');
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: var(--tc-font-family);
      background: var(--tc-background);
      color: var(--tc-text-primary);
      line-height: 1.4;
      -webkit-font-smoothing: antialiased;
    }
    .wrap {
      max-width: var(--tc-content-max-width);
      margin: 0 auto;
      padding: var(--tc-space-md);
    }
    .siteHeader {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: var(--tc-space-md) 0;
      border-bottom: 1px solid var(--tc-border);
    }
    .siteHeader a { color: var(--tc-text-primary); text-decoration: none; font-weight: 700; }
    h1 { font-size: 2rem; line-height: 1.2; margin: var(--tc-space-xl) 0 var(--tc-space-sm); }
    h2 { font-size: 1.5rem; margin: var(--tc-space-xl) 0 var(--tc-space-md); }
    h3 { font-size: 1.125rem; margin: 0; }
    .lead { font-size: 1.125rem; color: var(--tc-text-secondary); margin: 0 0 var(--tc-space-lg); }
    .appList, .statList { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--tc-space-md); }
    .appCard {
      background: var(--tc-surface);
      border: 1px solid var(--tc-border);
      border-radius: var(--tc-radius-md);
      padding: var(--tc-space-md);
    }
    .appCard__head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--tc-space-sm); }
    .appCard__address { margin: var(--tc-space-sm) 0 var(--tc-space-sm); font-weight: 600; }
    .appCard__desc { margin: 0 0 var(--tc-space-sm); color: var(--tc-text-secondary); }
    .appCard__meta { display: flex; flex-wrap: wrap; gap: var(--tc-space-md); align-items: center; font-size: 0.875rem; }
    .appCard__date { color: var(--tc-text-secondary); }
    .appLink { color: var(--tc-amber); text-decoration: none; font-weight: 600; }
    .appLink:hover { color: var(--tc-amber-hover); text-decoration: underline; }
    .status {
      display: inline-flex;
      align-items: center;
      border-radius: var(--tc-radius-full);
      padding: 2px var(--tc-space-sm);
      font-size: 0.8125rem;
      font-weight: 600;
      white-space: nowrap;
      color: var(--tc-status-default);
      border: 1px solid currentColor;
    }
    .status--permitted { color: var(--tc-status-permitted); }
    .status--conditions { color: var(--tc-status-conditions); }
    .status--rejected { color: var(--tc-status-rejected); }
    .status--withdrawn { color: var(--tc-status-withdrawn); }
    .status--appealed { color: var(--tc-status-appealed); }
    .statRow { display: flex; justify-content: space-between; background: var(--tc-surface); border: 1px solid var(--tc-border); border-radius: var(--tc-radius-md); padding: var(--tc-space-sm) var(--tc-space-md); }
    .statRow__count { font-weight: 700; }
    .explainer p { color: var(--tc-text-secondary); }
    .cta {
      margin: var(--tc-space-xl) 0;
      padding: var(--tc-space-lg);
      background: var(--tc-surface);
      border: 1px solid var(--tc-border);
      border-radius: var(--tc-radius-md);
      text-align: center;
    }
    .cta__button {
      display: inline-block;
      margin-top: var(--tc-space-md);
      padding: var(--tc-space-sm) var(--tc-space-lg);
      background: var(--tc-amber);
      color: var(--tc-text-on-accent);
      border-radius: var(--tc-radius-md);
      text-decoration: none;
      font-weight: 700;
    }
    .siteFooter { margin-top: var(--tc-space-xxl); padding: var(--tc-space-lg) 0; border-top: 1px solid var(--tc-border); color: var(--tc-text-secondary); font-size: 0.875rem; }
    .attribution { list-style: none; margin: 0 0 var(--tc-space-md); padding: 0; display: grid; gap: var(--tc-space-sm); }
    .siteFooter a { color: var(--tc-text-secondary); }`;
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
  const lead = escapeHtml(leadLine(data.areaName, data.total, data.totalCapped));
  const title = `Planning applications in ${area} | Town Crier`;
  const metaDescription = escapeHtml(
    `Recent planning applications in ${data.areaName}. See what is being built nearby and get push alerts the moment an application is submitted or decided in your area.`,
  );
  const jsonLd = buildJsonLd(data, canonical);
  const year = new Date().getFullYear();

  const applicationsList = data.applications.map(renderApplication).join('\n');
  const attribution = ATTRIBUTION_LINES.map(
    (line) => `        <li>${escapeHtml(line)}</li>`,
  ).join('\n');

  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
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
        <a href="/">Home</a>
      </header>
      <main>
        <h1>Planning applications in ${area}</h1>
        <p class="lead">${lead}</p>

${renderStats(data.applications)}

        <h2>Recent applications</h2>
        <ul class="appList">
${applicationsList}
        </ul>

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
          <a class="cta__button" href="${APP_DOWNLOAD_URL}" rel="noopener" target="_blank">Download on the App Store</a>
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
