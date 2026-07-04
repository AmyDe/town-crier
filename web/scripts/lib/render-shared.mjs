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
  statusDisplayLabel,
  aggregateBreakdown,
  aggregateStatusSummary,
  dataUpdatedLine,
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
 * @property {string | null} link           PlanIt permalink (always a reliable per-application record). No longer rendered
 *   as a per-card link (decision 6); kept only as a JSON-LD `url` fallback when no share URL can be built.
 * @property {string | null} url            council website (may be a generic portal page, not always a per-application deep link). No longer
 *   rendered as a per-card link (decision 6); kept only as a JSON-LD `url` fallback when no share URL can be built.
 */

const MAX_DESCRIPTION_LENGTH = 160;

/**
 * Shared status→colour vocabulary (decision 4, punch-list #794): only three
 * chip buckets across the SEO and share-page families. `Permitted` reads as
 * "granted" (green) and `Rejected` as "refused" (red); every other state —
 * the long tail (`Conditions`/"Granted with conditions", `Withdrawn`,
 * `Appealed`), a genuinely undecided wire state, an unrecognised future state,
 * or no state at all — buckets under the shared neutral chip. The resident-
 * facing *label* text is unaffected (still `statusDisplayLabel`); this only
 * consolidates which of the three canonical *colours* a state renders in.
 * @type {Record<string, 'granted' | 'refused'>}
 */
const STATUS_COLOR_MODIFIER = {
  Permitted: 'granted',
  Rejected: 'refused',
};

/**
 * @param {string | null | undefined} appState
 * @returns {'granted' | 'refused' | 'neutral'}
 */
function statusColorModifier(appState) {
  if (appState === null || appState === undefined) {
    return 'neutral';
  }
  return STATUS_COLOR_MODIFIER[appState] ?? 'neutral';
}

/**
 * @param {PlanningApplicationItem} app
 * @param {string} [authoritySlug] the page's authority slug; when present (and
 *   the app carries a ref), the whole card becomes a do-follow link to our own
 *   public share page for the application (decision 6). Every application on a
 *   page belongs to this one authority (authority pages read one partition;
 *   town pages scope the near query to a single authority), so the slug is
 *   correct for every card.
 * @returns {string}
 */
function renderApplication(app, authoritySlug) {
  const label = escapeHtml(statusDisplayLabel(app.appState));
  const modifier = statusColorModifier(app.appState);
  const description = escapeHtml(truncate(app.description, MAX_DESCRIPTION_LENGTH));

  // Address is the human hook (decision 5): it is the card's heading. The
  // council reference is demoted to small metadata underneath, and omitted
  // entirely when the app carries none.
  const address = escapeHtml(app.address);
  const ref = escapeHtml(app.name);

  // Per-card external links to PlanIt/the council are retired (decision 6):
  // the card's one click target is our own share page, surfaced as a real,
  // crawlable <a href> around the whole card plus a visible "View details"
  // affordance — never a JS-only click handler. Falls back to a plain,
  // non-linked card when no share URL can be built (no slug, or no ref).
  const share = shareUrl(authoritySlug, app.name);

  const body = `        <div class="appCard__head">
          <h3 class="appCard__address">${address}</h3>
          <span class="status status--${modifier}">${label}</span>
        </div>
        ${ref ? `<p class="appCard__ref">${ref}</p>` : ''}
        <p class="appCard__desc">${description}</p>
        <div class="appCard__meta">
          ${share ? `<span class="appCard__cta">View details →</span>` : ''}
        </div>`;

  if (share) {
    return `      <li class="appCard">
        <a class="appCard__link" href="${escapeHtml(share)}">
${body}
        </a>
      </li>`;
  }

  return `      <li class="appCard">
${body}
      </li>`;
}

/**
 * Render the recent-applications list body (the `<li>` cards joined by newlines).
 *
 * @param {PlanningApplicationItem[]} applications
 * @param {string} [authoritySlug] the page's authority slug, threaded through so
 *   each card can link to its share page. Omitted (or no ref on the app) -> the
 *   card renders without a link at all (decision 6 retired the external
 *   PlanIt/council per-card links, so there is no other href to fall back to).
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
 * Render the compact "Status breakdown" strip (tc-r4n9.3, punch-list #794
 * Phase 3): one line of three headline buckets (Granted / Refused /
 * Undecided) plus the total, using the SAME `.status--granted` /
 * `.status--refused` / `.status--neutral` chip vocabulary and colours as the
 * per-card chips (decision 4) for visual consistency between the aggregate
 * summary and the individual cards. Replaces the old one-row-per-appState
 * `renderStats` list, which advertised every long-tail state as its own
 * top-level row.
 *
 * Any state that doesn't fit the three headline buckets (the long tail —
 * `Conditions`/"Granted with conditions", `Withdrawn`, `Appealed`, `Referred`,
 * `Unresolved`, any future/unrecognised state) is folded behind a `<details>`
 * disclosure labelled "Other (N)" instead of being enumerated top-level.
 * Omitted entirely when there is no long tail at all.
 *
 * @param {ReadonlyArray<{ appState: string | null, count: number }>} statusBreakdown
 * @returns {string}
 */
export function renderStatusSummary(statusBreakdown) {
  const { granted, refused, undecided, total, other } = aggregateStatusSummary(statusBreakdown);
  const otherTotal = other.reduce((sum, o) => sum + o.count, 0);
  const otherDisclosure =
    otherTotal > 0
      ? `
      <details class="statusSummary__other">
        <summary>Other (${otherTotal})</summary>
        <ul class="statusSummary__otherList">
${other
  .map(
    (o) =>
      `          <li><span class="statusSummary__otherLabel">${escapeHtml(o.label)}</span><span class="statusSummary__otherCount">${o.count}</span></li>`,
  )
  .join('\n')}
        </ul>
      </details>`
      : '';
  return `    <section class="statusSummary" aria-label="Application status summary">
      <h2 class="statusSummary__heading">Status breakdown</h2>
      <div class="statusSummary__strip">
        <span class="statusSummary__item status status--granted">${granted} Granted</span>
        <span class="statusSummary__item status status--refused">${refused} Refused</span>
        <span class="statusSummary__item status status--neutral">${undecided} Undecided</span>
        <span class="statusSummary__total">${total} total</span>
      </div>${otherDisclosure}
    </section>`;
}

/**
 * Render the single "Data updated {date}" line (tc-r4n9.3, punch-list #794
 * Phase 3) that replaces the old per-card "Last updated {date}" line, which
 * repeated the same handful of snapshot dates once per card (up to 30 times
 * on a full page) under a "Recent applications" heading — reading as
 * snapshot staleness rather than a freshness signal. Placed once, near the
 * H1, using the freshest `lastDifferent` among the applications actually
 * shown. Returns '' (renders nothing) when no shown application carries a
 * parseable date.
 *
 * @param {ReadonlyArray<{ lastDifferent?: string | null }>} applications
 * @returns {string}
 */
export function renderDataUpdated(applications) {
  const line = dataUpdatedLine(applications);
  return line ? `<p class="dataUpdated">${escapeHtml(line)}</p>` : '';
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
 * Town Crier design tokens — light by default, dark via prefers-color-scheme —
 * mirroring `src/styles/tokens.css`. Light-first (rather than gating the light
 * values behind a media query) means a renderer/webview/reader-mode/print
 * pipeline that never evaluates prefers-color-scheme still gets the paper-like
 * default, not the dark palette (tc-r4n9.1).
 *
 * @returns {string}
 */
export function pageStyles() {
  return `    :root {
      /* Background scale converged with the share-page family (see
         api-go/internal/sharepage/templates/styles.gohtml): a warm cream field
         (--tc-background: #FAF8F5) behind a white card (--tc-surface:
         #FFFFFF) in light mode, so SEO pages and share pages read as one
         product rather than two differently-themed properties. */
      --tc-amber: #D4910A;
      --tc-amber-hover: #B87A08;
      --tc-background: #FAF8F5;
      --tc-surface: #FFFFFF;
      --tc-text-primary: #1C1917;
      --tc-text-secondary: #6B6560;
      --tc-text-on-accent: #FFFFFF;
      /* Shared status chip vocabulary (decision 4, punch-list #794): three
         canonical buckets — granted (green), refused (red), neutral (the
         undecided/long-tail catch-all) — converged with the design-language
         tcStatusApproved/tcStatusRefused/tcTextSecondary tokens, replacing the
         previous five-way ad-hoc per-appState palette. The *-bg tokens are the
         foreground colour at 15% opacity, for the filled badge style. */
      --tc-status-granted: #1A7D37;
      --tc-status-granted-bg: #1A7D3726;
      --tc-status-refused: #C42B2B;
      --tc-status-refused-bg: #C42B2B26;
      --tc-status-neutral: #6B6560;
      --tc-status-neutral-bg: #6B656026;
      --tc-border: #E8E4DF;
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
    @media (prefers-color-scheme: dark) {
      :root {
        --tc-amber: #E9A620;
        --tc-amber-hover: #F0B83A;
        --tc-background: #1A1A1E;
        --tc-surface: #242428;
        --tc-text-primary: #F1EFE9;
        --tc-text-secondary: #9B9590;
        --tc-text-on-accent: #1C1917;
        --tc-status-granted: #34C759;
        --tc-status-granted-bg: #34C75926;
        --tc-status-refused: #FF453A;
        --tc-status-refused-bg: #FF453A26;
        --tc-status-neutral: #9B9590;
        --tc-status-neutral-bg: #9B959026;
        --tc-border: #3A3A3F;
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
    /* Single freshness line (tc-r4n9.3), placed near the H1. */
    .dataUpdated { margin: 0 0 var(--tc-space-sm); font-size: 0.875rem; color: var(--tc-text-secondary); }
    .lead { font-size: 1.125rem; color: var(--tc-text-secondary); margin: 0 0 var(--tc-space-lg); }
    .appList { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--tc-space-md); }
    .appCard {
      background: var(--tc-surface);
      border: 1px solid var(--tc-border);
      border-radius: var(--tc-radius-md);
      padding: var(--tc-space-md);
    }
    /* The whole card is the share-page click target (decision 6): a real,
       crawlable <a href> wraps every card that has a share URL, styled to
       read as plain card content rather than a traditional blue link. */
    .appCard__link { display: block; color: inherit; text-decoration: none; }
    .appCard__link:focus-visible {
      outline: 2px solid var(--tc-amber);
      outline-offset: 2px;
      border-radius: var(--tc-radius-md);
    }
    .appCard__link:hover .appCard__address { color: var(--tc-amber); }
    .appCard__link:hover .appCard__cta { color: var(--tc-amber-hover); text-decoration: underline; }
    .appCard__head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--tc-space-sm); }
    /* Address is the human hook (decision 5): the card headline. */
    .appCard__address { margin: 0; font-weight: 600; overflow-wrap: anywhere; }
    /* The council reference is demoted to small metadata under the headline. */
    .appCard__ref { margin: var(--tc-space-sm) 0; font-size: 0.8125rem; color: var(--tc-text-secondary); overflow-wrap: anywhere; }
    .appCard__desc { margin: 0 0 var(--tc-space-sm); color: var(--tc-text-secondary); }
    .appCard__meta { display: flex; flex-wrap: wrap; gap: var(--tc-space-md); align-items: center; font-size: 0.875rem; }
    /* Visible share-page affordance (decision 6) — a real anchor, not a
       JS-only click handler; this is the text cue, the href is the whole card. */
    .appCard__cta { color: var(--tc-amber); font-weight: 600; }
    .status {
      display: inline-flex;
      align-items: center;
      border-radius: var(--tc-radius-full);
      padding: 2px var(--tc-space-sm);
      font-size: 0.8125rem;
      font-weight: 600;
      white-space: nowrap;
    }
    /* Shared filled-chip vocabulary (decision 4): three canonical buckets,
       background = foreground colour at 15% opacity, converged with the
       design-language Status Badge pattern. */
    .status--granted { color: var(--tc-status-granted); background: var(--tc-status-granted-bg); }
    .status--refused { color: var(--tc-status-refused); background: var(--tc-status-refused-bg); }
    .status--neutral { color: var(--tc-status-neutral); background: var(--tc-status-neutral-bg); }
    /* Compact "Status breakdown" strip (tc-r4n9.3): one row of chip-style
       headline buckets (reusing the .status--granted/refused/neutral chip
       vocabulary above) plus the total, with any long tail folded behind the
       .statusSummary__other <details> instead of a one-row-per-state list. */
    .statusSummary__strip { display: flex; flex-wrap: wrap; align-items: center; gap: var(--tc-space-sm); }
    .statusSummary__total { color: var(--tc-text-secondary); font-size: 0.875rem; }
    .statusSummary__other { margin-top: var(--tc-space-sm); color: var(--tc-text-secondary); font-size: 0.875rem; }
    .statusSummary__other summary { cursor: pointer; color: var(--tc-amber); font-weight: 600; }
    .statusSummary__otherList { list-style: none; margin: var(--tc-space-sm) 0 0; padding: 0; display: grid; gap: var(--tc-space-sm); }
    .statusSummary__otherList li { display: flex; justify-content: space-between; gap: var(--tc-space-md); }
    .statusSummary__otherCount { font-weight: 700; }
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
