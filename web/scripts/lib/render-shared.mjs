/**
 * Building blocks shared by the authority-page renderer (`render-page.mjs`) and
 * the town-page renderer (`render-town-page.mjs`). Both pages are the same
 * hydration-free static template — only the title, canonical URL, breadcrumb and
 * a little evergreen copy differ — so the application cards, status stats,
 * attribution footer and the inline stylesheet live here once.
 *
 * Everything emitted here is destined for raw HTML, so data-derived strings are
 * passed through `escapeHtml` before interpolation.
 */

import { ATTRIBUTION_LINES, shareUrl } from './constants.mjs';
import {
  escapeHtml,
  truncate,
  formatDate,
  statusDisplayLabel,
  aggregateBreakdown,
} from './format.mjs';

/**
 * @typedef {Object} PlanningApplicationItem
 * @property {string} uid
 * @property {string} name                   PlanIt `planit_name`; the share page's lookup key (the share-URL ref)
 * @property {string} address
 * @property {string} description
 * @property {string | null} appState
 * @property {string | null} startDate      yyyy-MM-dd
 * @property {string} lastDifferent         ISO-8601 with offset; the DESC sort key
 * @property {string | null} link           PlanIt permalink (always a reliable per-application record)
 * @property {string | null} url            council website (may be a generic portal page, not always a per-application deep link)
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
 * @param {string} [authoritySlug] the page's authority slug; when present (and the
 *   app carries a ref), the card's reference heading becomes a do-follow link to
 *   our own public share page for the application. Every application on a page
 *   belongs to this one authority (authority pages read one partition; town pages
 *   scope the near query to a single authority), so the slug is correct for every
 *   card.
 * @returns {string}
 */
function renderApplication(app, authoritySlug) {
  const label = escapeHtml(statusDisplayLabel(app.appState));
  const modifier = statusModifier(app.appState);
  // The visible date is lastDifferent (the bounded read's DESC sort key), so the
  // dates read top-to-bottom in the same order the cards are listed.
  const date = formatDate(app.lastDifferent);
  const description = escapeHtml(truncate(app.description, MAX_DESCRIPTION_LENGTH));

  // The reference heading is the card's title and doubles as the link to our own
  // share page for the application — a do-follow, same-tab internal link we want
  // crawled and clicked. It stays plain text when the page has no authority slug
  // or the app carries no ref. The external PlanIt/council links stay in the meta
  // row; keeping our link on the title (not a third meta link) avoids a lopsided,
  // wrapping action row.
  const ref = escapeHtml(app.name);
  const share = shareUrl(authoritySlug, app.name);
  const heading = share
    ? `<a class="appCard__refLink" href="${escapeHtml(share)}">${ref}</a>`
    : ref;

  const links = [];
  if (app.link) {
    links.push(
      `<a class="appLink" href="${escapeHtml(app.link)}" rel="nofollow noopener" target="_blank">View full record on PlanIt</a>`,
    );
  }
  if (app.url) {
    links.push(
      `<a class="appLink" href="${escapeHtml(app.url)}" rel="nofollow noopener" target="_blank">View on the council website</a>`,
    );
  }

  return `      <li class="appCard">
        <div class="appCard__head">
          <h3 class="appCard__ref">${heading}</h3>
          <span class="status status--${modifier}">${label}</span>
        </div>
        <p class="appCard__address">${escapeHtml(app.address)}</p>
        <p class="appCard__desc">${description}</p>
        <div class="appCard__meta">
          ${date ? `<span class="appCard__date">Last updated ${escapeHtml(date)}</span>` : ''}
          ${links.join('\n          ')}
        </div>
      </li>`;
}

/**
 * Render the recent-applications list body (the `<li>` cards joined by newlines).
 *
 * @param {PlanningApplicationItem[]} applications
 * @param {string} [authoritySlug] the page's authority slug, threaded through so
 *   each card can link to its share page. Omitted -> no share links (the external
 *   PlanIt/council links still render).
 * @returns {string}
 */
export function renderApplicationsList(applications, authoritySlug) {
  return applications.map((app) => renderApplication(app, authoritySlug)).join('\n');
}

/**
 * Render the status-breakdown block from the server-provided per-`appState`
 * distribution (computed over the bounded read), folded into resident-facing
 * labels. This is intentionally NOT derived from the handful of cards rendered
 * on the page, so the counts reflect the wider bounded set.
 *
 * @param {ReadonlyArray<{ appState: string | null, count: number }>} statusBreakdown
 * @returns {string}
 */
export function renderStats(statusBreakdown) {
  const rows = aggregateBreakdown(statusBreakdown)
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
 * Render the mandatory data-attribution `<li>` items (ADR 0006). Required on
 * every public page. Defaults to the base {@link ATTRIBUTION_LINES}; callers can
 * supply an extended list (e.g. town pages add the ONS/NRS gazetteer credits)
 * without the authority page picking up sources it doesn't use.
 *
 * @param {readonly string[]} [lines]
 * @returns {string}
 */
export function renderAttributionList(lines = ATTRIBUTION_LINES) {
  return lines
    .map((line) => `        <li>${escapeHtml(line)}</li>`)
    .join('\n');
}

/**
 * Inline stylesheet for the standalone static pages. Self-contained (no external
 * CSS request) for first-byte readability and strong Core Web Vitals. Uses the
 * Town Crier design tokens — dark by default, light via prefers-color-scheme —
 * mirroring `src/styles/tokens.css`.
 *
 * @returns {string}
 */
export function pageStyles() {
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
      flex-wrap: wrap;
      gap: var(--tc-space-sm);
      padding: var(--tc-space-md) 0;
      border-bottom: 1px solid var(--tc-border);
    }
    .siteHeader a { color: var(--tc-text-primary); text-decoration: none; font-weight: 700; }
    .siteHeader__nav { display: flex; align-items: center; gap: var(--tc-space-md); }
    .siteHeader a.siteHeader__cta {
      padding: var(--tc-space-sm) var(--tc-space-md);
      background: var(--tc-amber);
      color: var(--tc-text-on-accent);
      border-radius: var(--tc-radius-md);
    }
    .siteHeader a.siteHeader__cta:hover { background: var(--tc-amber-hover); }
    .breadcrumb { margin: var(--tc-space-md) 0 0; font-size: 0.875rem; color: var(--tc-text-secondary); }
    .breadcrumb ol { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: var(--tc-space-sm); }
    .breadcrumb li::after { content: '/'; margin-left: var(--tc-space-sm); color: var(--tc-text-secondary); }
    .breadcrumb li:last-child::after { content: ''; margin: 0; }
    .breadcrumb a { color: var(--tc-text-secondary); text-decoration: none; }
    .breadcrumb a:hover { color: var(--tc-amber); text-decoration: underline; }
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
    .appCard__ref { overflow-wrap: anywhere; }
    .appCard__refLink { color: inherit; text-decoration: none; }
    .appCard__refLink:hover { color: var(--tc-amber); text-decoration: underline; }
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
    .townLinks__list { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: var(--tc-space-sm); }
    .townLinks__list a {
      display: inline-block;
      padding: var(--tc-space-sm) var(--tc-space-md);
      background: var(--tc-surface);
      border: 1px solid var(--tc-border);
      border-radius: var(--tc-radius-full);
      color: var(--tc-amber);
      text-decoration: none;
      font-weight: 600;
    }
    .townLinks__list a:hover { color: var(--tc-amber-hover); border-color: var(--tc-amber); }
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
