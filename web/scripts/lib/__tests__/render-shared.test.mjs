import { describe, it, expect } from 'vitest';
import {
  renderApplicationsList,
  renderAttributionList,
  renderStatusSummary,
  renderDataUpdated,
  renderInlineCta,
  renderMidListCta,
  renderQrBlock,
  renderPlanningCrossLinks,
  pageStyles,
} from '../render-shared.mjs';
import { ATTRIBUTION_LINES } from '../constants.mjs';

const PLANIT_CAPTION = 'View full record on PlanIt';
const COUNCIL_CAPTION = 'View on the council website';

const SLUG = 'basingstoke-and-deane';
// The share ref is the app's `name` (planit_name), NOT its uid; slashes are kept.
const SHARE_HREF = `https://share.towncrierapp.uk/a/${SLUG}/26/0001/FUL`;
const PLANIT_HREF = 'https://planit.org.uk/planapplic/26-0001-FUL';
const COUNCIL_HREF = 'https://www.basingstoke.gov.uk/planning/26-0001-FUL';

/**
 * @param {Partial<import('../render-shared.mjs').PlanningApplicationItem>} [overrides]
 * @returns {import('../render-shared.mjs').PlanningApplicationItem}
 */
function application(overrides = {}) {
  return {
    uid: 'BDB/2026/0001',
    name: '26/0001/FUL',
    address: '12 Mill Road, Basingstoke, RG21 1AA',
    description: 'Erection of two-storey rear extension',
    appState: 'Permitted',
    startDate: '2026-01-15',
    lastDifferent: '2026-06-12T09:30:00+00:00',
    link: PLANIT_HREF,
    url: COUNCIL_HREF,
    ...overrides,
  };
}

describe('renderApplicationsList ledger row structure (decision 5: address is the human hook)', () => {
  it('renders the address as the <h3> headline', () => {
    const html = renderApplicationsList([application()], SLUG);
    expect(html).toContain(
      '<h3 class="ledgerRow__address">12 Mill Road, Basingstoke, RG21 1AA</h3>',
    );
  });

  it('demotes the council reference to small metadata text, not a heading', () => {
    const html = renderApplicationsList([application()], SLUG);
    expect(html).toContain('<p class="ledgerRow__ref">26/0001/FUL</p>');
    expect(html).not.toContain('<h3 class="ledgerRow__ref">');
    expect(html).not.toContain('ledgerRow__refLink');
  });

  it('omits the reference metadata line entirely when the app carries no ref', () => {
    const html = renderApplicationsList([application({ name: '' })], SLUG);
    expect(html).not.toContain('ledgerRow__ref');
  });

  it('HTML-escapes the address', () => {
    const html = renderApplicationsList(
      [application({ address: '<script>alert(1)</script>' })],
      SLUG,
    );
    expect(html).not.toContain('<script>alert(1)</script>');
    expect(html).toContain('&lt;script&gt;alert(1)&lt;/script&gt;');
  });
});

describe('renderApplicationsList description truncation (word boundary)', () => {
  it('truncates a long description on a word boundary with an ellipsis, never mid-word', () => {
    const longDescription =
      'Erection of a two-storey rear extension and associated landscaping works to an existing dwelling house with a new detached garage and altered vehicular access from the existing driveway';
    const html = renderApplicationsList(
      [application({ description: longDescription })],
      SLUG,
    );
    const match = html.match(/<p class="ledgerRow__desc">([^<]*)<\/p>/);
    expect(match).not.toBeNull();
    const rendered = match[1];
    expect(rendered.endsWith('…')).toBe(true);
    const withoutEllipsis = rendered.slice(0, -1);
    expect(longDescription.startsWith(withoutEllipsis)).toBe(true);
    // No trailing space before the ellipsis, and the next source character is a
    // space (or end of string) — proof the cut landed on a word boundary.
    expect(withoutEllipsis.endsWith(' ')).toBe(false);
    const nextChar = longDescription[withoutEllipsis.length];
    expect(nextChar === ' ' || nextChar === undefined).toBe(true);
  });

  it('leaves a short description unchanged with no ellipsis', () => {
    const html = renderApplicationsList([application()], SLUG);
    expect(html).toContain(
      '<p class="ledgerRow__desc">Erection of two-storey rear extension</p>',
    );
  });
});

describe('renderApplicationsList share-page affordance (decision 6)', () => {
  it('wraps the whole row in a real anchor pointing at the share page', () => {
    const html = renderApplicationsList([application()], SLUG);
    expect(html).toContain(`<a class="ledgerRow__link" href="${SHARE_HREF}">`);
  });

  it('includes a visible "View details" affordance inside the row link', () => {
    const html = renderApplicationsList([application()], SLUG);
    expect(html).toContain('<span class="ledgerRow__cta">View details →</span>');
  });

  it('never relies on a JS-only click handler for the share-page target', () => {
    const html = renderApplicationsList([application()], SLUG);
    expect(html).not.toContain('onclick');
    expect(html).toContain(`href="${SHARE_HREF}"`);
  });

  it('percent-encodes unsafe ref characters per segment while preserving slashes', () => {
    const html = renderApplicationsList(
      [application({ name: 'DC/2026/0001 A&B' })],
      SLUG,
    );
    expect(html).toContain(
      `href="https://share.towncrierapp.uk/a/${SLUG}/DC/2026/0001%20A%26B"`,
    );
  });

  it('falls back to a non-linked row when no share URL can be built (no slug supplied)', () => {
    const html = renderApplicationsList([application()]);
    expect(html).not.toContain('ledgerRow__link');
    expect(html).not.toContain('View details');
    expect(html).not.toContain('share.towncrierapp.uk');
  });

  it('falls back to a non-linked row when the app carries no ref', () => {
    const html = renderApplicationsList([application({ name: '' })], SLUG);
    expect(html).not.toContain('ledgerRow__link');
    expect(html).not.toContain('View details');
    expect(html).not.toContain('share.towncrierapp.uk');
  });
});

describe('renderApplicationsList external link removal (decision 6)', () => {
  it('renders no per-row links to PlanIt or the council website', () => {
    const html = renderApplicationsList([application()], SLUG);
    expect(html).not.toContain(PLANIT_CAPTION);
    expect(html).not.toContain(COUNCIL_CAPTION);
    expect(html).not.toContain(PLANIT_HREF);
    expect(html).not.toContain(COUNCIL_HREF);
    expect(html).not.toContain('class="appLink"');
  });

  it('renders no external links even when the app carries neither link nor url', () => {
    const html = renderApplicationsList(
      [application({ link: null, url: null })],
      SLUG,
    );
    expect(html).not.toContain('class="appLink"');
  });
});

describe('renderApplicationsList status stamp vocabulary (decision 4: shared vocabulary; Public Notice stamp language)', () => {
  it('colours a Granted (Permitted) application with the granted stamp and pairs an icon with the label (accessibility invariant)', () => {
    const html = renderApplicationsList(
      [application({ appState: 'Permitted' })],
      SLUG,
    );
    expect(html).toContain('class="stamp stamp--granted"');
    expect(html).toContain('<span class="stamp__label">Granted</span>');
    expect(html).toContain('class="stamp__icon"');
    expect(html).toMatch(/class="stamp__icon"[^>]*aria-hidden="true"/);
  });

  it('colours a Refused (Rejected) application with the refused stamp', () => {
    const html = renderApplicationsList(
      [application({ appState: 'Rejected' })],
      SLUG,
    );
    expect(html).toContain('class="stamp stamp--refused"');
    expect(html).toContain('<span class="stamp__label">Refused</span>');
  });

  it.each([
    ['Conditions', 'Granted with conditions'],
    ['Withdrawn', 'Withdrawn'],
    ['Appealed', 'Appealed'],
    ['Undecided', 'Undecided'],
    [null, 'Unknown'],
  ])(
    'buckets the long-tail / undecided state %s under the shared neutral stamp',
    (appState, label) => {
      const html = renderApplicationsList(
        [application({ appState })],
        SLUG,
      );
      expect(html).toContain('class="stamp stamp--neutral"');
      expect(html).toContain(`<span class="stamp__label">${label}</span>`);
    },
  );

  it('only ever emits the three canonical stamp modifiers, never the old per-state ones', () => {
    const states = [
      'Permitted',
      'Rejected',
      'Conditions',
      'Withdrawn',
      'Appealed',
      null,
      'SomeFutureState',
    ];
    const html = renderApplicationsList(
      states.map((appState) => application({ appState })),
      SLUG,
    );
    const modifiers = [...html.matchAll(/class="stamp stamp--(\w+)"/g)].map(
      (m) => m[1],
    );
    expect(new Set(modifiers)).toEqual(new Set(['granted', 'refused', 'neutral']));
  });

  it('never emits the old filled-chip class names', () => {
    const html = renderApplicationsList([application()], SLUG);
    expect(html).not.toContain('class="status status--');
    expect(html).not.toContain('appCard');
  });
});

describe('renderApplicationsList Started/Decided date line (tc-s0yf, GH #819)', () => {
  it('renders "Decided <date>" when decidedDate is set, even if startDate is also set (decided takes precedence)', () => {
    const html = renderApplicationsList(
      [application({ startDate: '2021-05-28', decidedDate: '2021-07-09' })],
      SLUG,
    );
    expect(html).toContain('<p class="ledgerRow__date">Decided 9 Jul 2021</p>');
    expect(html).not.toContain('Started 28 May 2021');
  });

  it('renders "Started <date> · Awaiting decision" when only startDate is set', () => {
    const html = renderApplicationsList(
      [application({ startDate: '2026-07-04', decidedDate: null })],
      SLUG,
    );
    expect(html).toContain(
      '<p class="ledgerRow__date">Started 4 Jul 2026 · Awaiting decision</p>',
    );
  });

  it('renders "Decided <date>" when only decidedDate is set (no startDate)', () => {
    const html = renderApplicationsList(
      [application({ startDate: null, decidedDate: '2021-07-09' })],
      SLUG,
    );
    expect(html).toContain('<p class="ledgerRow__date">Decided 9 Jul 2021</p>');
  });

  it('renders no date line at all when neither date is present, without crashing or emitting "undefined"/"Invalid Date"', () => {
    const html = renderApplicationsList(
      [application({ startDate: null, decidedDate: null })],
      SLUG,
    );
    expect(html).not.toContain('ledgerRow__date');
    expect(html).not.toContain('undefined');
    expect(html).not.toContain('Invalid Date');
  });

  it('handles a missing decidedDate key entirely (not just null) without crashing', () => {
    const app = application({ startDate: '2026-01-15' });
    delete app.decidedDate;
    const html = renderApplicationsList([app], SLUG);
    expect(html).toContain(
      '<p class="ledgerRow__date">Started 15 Jan 2026 · Awaiting decision</p>',
    );
    expect(html).not.toContain('undefined');
  });
});

describe('renderStatusSummary (tc-r4n9.3: compact Granted/Refused/Undecided strip, Public Notice stamp language)', () => {
  const BREAKDOWN = [
    { appState: 'Permitted', count: 20 },
    { appState: 'Rejected', count: 12 },
    { appState: 'Undecided', count: 8 },
    { appState: null, count: 2 },
  ];

  it('renders a single compact strip with the three headline buckets and the total', () => {
    const html = renderStatusSummary(BREAKDOWN);
    expect(html).toContain('<h2 class="statusSummary__heading">Status breakdown</h2>');
    expect(html).toMatch(/20[\s\S]{0,20}Granted/);
    expect(html).toMatch(/12[\s\S]{0,20}Refused/);
    expect(html).toMatch(/10[\s\S]{0,20}Undecided/);
    expect(html).toMatch(/42[\s\S]{0,20}total/);
  });

  it('reuses the shared stamp vocabulary/colours for the strip items, with icon + label', () => {
    const html = renderStatusSummary(BREAKDOWN);
    expect(html).toContain('stamp--granted');
    expect(html).toContain('stamp--refused');
    expect(html).toContain('stamp--neutral');
    expect((html.match(/class="stamp__icon"/g) ?? []).length).toBeGreaterThanOrEqual(3);
  });

  it('does not enumerate every top-level status as its own row (compact, not a wall of lines)', () => {
    const html = renderStatusSummary(BREAKDOWN);
    // No long tail in this breakdown, so no "Other" disclosure at all.
    expect(html).not.toContain('statusSummary__other');
  });

  it('folds long-tail states behind a details disclosure instead of listing them top-level', () => {
    const withLongTail = [
      ...BREAKDOWN,
      { appState: 'Conditions', count: 5 },
      { appState: 'Withdrawn', count: 3 },
    ];
    const html = renderStatusSummary(withLongTail);
    expect(html).toContain('<details class="statusSummary__other">');
    expect(html).toContain('<summary>Other (8)</summary>');
    expect(html).toContain('Granted with conditions');
    expect(html).toContain('Withdrawn');
    // The long-tail states are inside the disclosure, not top-level stamps.
    expect(html).not.toMatch(/stamp--\w+">\s*5 Granted with conditions/);
  });

  it('omits the Other disclosure entirely when there is no long tail', () => {
    const html = renderStatusSummary(BREAKDOWN);
    expect(html).not.toContain('<details');
    expect(html).not.toContain('Other (');
  });

  it('HTML-escapes long-tail labels', () => {
    const html = renderStatusSummary([
      { appState: '<script>alert(1)</script>', count: 1 },
    ]);
    expect(html).not.toContain('<script>alert(1)</script>');
    expect(html).toContain('&lt;script&gt;alert(1)&lt;/script&gt;');
  });
});

describe('renderDataUpdated (tc-r4n9.3: single line replacing per-card repetition)', () => {
  it('renders one "Data updated" line from the freshest application date', () => {
    const html = renderDataUpdated([
      { lastDifferent: '2026-06-12T09:30:00+00:00' },
      { lastDifferent: '2026-06-15T10:00:00+00:00' },
    ]);
    expect(html).toBe('<p class="dataUpdated">Data updated 15 Jun 2026</p>');
  });

  it('renders nothing when no application carries a parseable date', () => {
    expect(renderDataUpdated([])).toBe('');
    expect(renderDataUpdated([{ lastDifferent: null }])).toBe('');
  });
});

describe('renderInlineCta (tc-r4n9.3: alerts CTA pulled above the list)', () => {
  it('renders a real anchor to the App Store with the area-specific alert copy', () => {
    const html = renderInlineCta('Basingstoke and Deane', 'https://apps.apple.com/x?ct=seo-lpa');
    expect(html).toContain('class="ctaInline"');
    expect(html).toContain('href="https://apps.apple.com/x?ct=seo-lpa"');
    expect(html).toContain('Get push alerts for Basingstoke and Deane');
  });

  it('HTML-escapes the area name', () => {
    const html = renderInlineCta('<script>alert(1)</script>', 'https://apps.apple.com/x');
    expect(html).not.toContain('<script>alert(1)</script>');
    expect(html).toContain('&lt;script&gt;alert(1)&lt;/script&gt;');
  });

  it('is a real crawlable link, never a JS-only click handler', () => {
    const html = renderInlineCta('Truro', 'https://apps.apple.com/x');
    expect(html).not.toContain('onclick');
    expect(html).toContain('rel="noopener"');
    expect(html).toContain('target="_blank"');
  });
});

describe('renderAttributionList', () => {
  it('defaults to the base ATTRIBUTION_LINES (one escaped <li> per line)', () => {
    const html = renderAttributionList();
    for (const line of ATTRIBUTION_LINES) {
      expect(html).toContain(`<li>${line}</li>`);
    }
    expect((html.match(/<li>/g) ?? []).length).toBe(ATTRIBUTION_LINES.length);
  });

  it('renders a caller-supplied list of lines, so a page can extend the base set', () => {
    const html = renderAttributionList([
      'Line one',
      'Line two & <b>bold</b>',
    ]);
    expect(html).toContain('<li>Line one</li>');
    // HTML in a line is escaped so data can never inject markup.
    expect(html).toContain('<li>Line two &amp; &lt;b&gt;bold&lt;/b&gt;</li>');
    expect((html.match(/<li>/g) ?? []).length).toBe(2);
  });
});

describe('renderPlanningCrossLinks (tc-3ht16, GH #990 slice 1: un-orphan /planning/towns)', () => {
  it('renders a real, crawlable link to /planning/towns', () => {
    const html = renderPlanningCrossLinks();
    expect(html).toContain('class="crossLinks"');
    expect(html).toContain('<a class="crossLinks__link" href="/planning/towns">');
    expect(html).not.toContain('onclick');
  });

  it('takes no arguments, so both the hub and every authority page render byte-identical markup', () => {
    expect(renderPlanningCrossLinks()).toBe(renderPlanningCrossLinks());
  });
});

describe('pageStyles cross-link (tc-3ht16)', () => {
  it('styles the towns-index cross-link as an amber text link, not a filled pill', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.crossLinks__link \{[^}]*color: var\(--tc-amber\)/);
    expect(css).not.toMatch(/\.crossLinks__link \{[^}]*background: var\(--tc-amber\)/);
  });
});

describe('pageStyles masthead (double rule, small-caps wordmark, brass text CTA)', () => {
  it('gives the wordmark a small-caps, letterspaced treatment', () => {
    const css = pageStyles();
    expect(css).toContain('.siteHeader__wordmark');
    expect(css).toMatch(/\.siteHeader__wordmark \{[^}]*font-variant: small-caps/);
  });

  it('restyles the header CTA as a plain brass text link, not a filled button', () => {
    const css = pageStyles();
    expect(css).toMatch(/a\.siteHeader__cta \{[^}]*color: var\(--tc-amber\)/);
    // No filled-pill chrome left on the header CTA.
    expect(css).not.toMatch(/a\.siteHeader__cta \{[^}]*background: var\(--tc-amber\)/);
  });

  it('renders a double rule beneath the masthead — a 2.5px heavy rule over a 1px hairline', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.siteHeader__ruleHeavy \{[^}]*height: 2\.5px/);
    expect(css).toMatch(/\.siteHeader__ruleHeavy \{[^}]*var\(--tc-text-primary\)/);
    expect(css).toMatch(/\.siteHeader__ruleHairline \{[^}]*height: 1px/);
    expect(css).toMatch(/\.siteHeader__ruleHairline \{[^}]*var\(--tc-border\)/);
  });
});

describe('pageStyles breadcrumb (mono small-caps dateline)', () => {
  it('sets the breadcrumb in the mono font role, small-caps, letterspaced', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.breadcrumb \{[^}]*font-family: var\(--tc-font-mono\)/);
    expect(css).toMatch(/\.breadcrumb \{[^}]*font-variant: small-caps/);
  });

  it('separates dateline segments with a right-pointing angle quote, not a slash', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.breadcrumb li::after \{[^}]*content: '›'/);
  });
});

describe('pageStyles H1 (display font token)', () => {
  it('sets h1 to the display font token (sans stack, no self-hosted display face)', () => {
    const css = pageStyles();
    expect(css).toMatch(/^\s*h1 \{[^}]*font-family: var\(--tc-font-display\)/m);
  });
});

describe('pageStyles font self-hosting (zero external font requests, no Fraunces, tc-b3nki.7)', () => {
  it('declares no @font-face block for Fraunces', () => {
    const css = pageStyles();
    expect(css).not.toMatch(/font-family: 'Fraunces'/);
  });

  it('still declares the Inter @font-face block, same-origin woff2', () => {
    const css = pageStyles();
    expect(css).toMatch(
      /@font-face \{[^}]*font-family: 'Inter';[^}]*src: url\('\/fonts\/inter-latin\.woff2'\) format\('woff2'\);/,
    );
  });

  it('uses font-display: swap on every @font-face block', () => {
    const css = pageStyles();
    const blocks = css.match(/@font-face \{[^}]*\}/g) ?? [];
    expect(blocks.length).toBeGreaterThan(0);
    for (const block of blocks) {
      expect(block).toContain('font-display: swap');
    }
  });

  it('never references a third-party font host', () => {
    const css = pageStyles();
    expect(css).not.toContain('https://fonts');
    expect(css).not.toContain('fonts.googleapis.com');
    expect(css).not.toContain('fonts.gstatic.com');
  });
});

describe('pageStyles ledger (section heading, rows, stamps)', () => {
  it('opens the ledger with a small-caps brass label over a heavy 2px rule', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.ledger__heading \{[^}]*font-variant: small-caps/);
    expect(css).toMatch(/\.ledger__heading \{[^}]*color: var\(--tc-amber\)/);
    expect(css).toMatch(/\.ledger__heading \{[^}]*border-top: 2px solid var\(--tc-text-primary\)/);
  });

  it('separates ledger rows with a hairline rule, not a boxed card border', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.ledgerRow \{[^}]*border-bottom: 1px solid var\(--tc-border\)/);
    expect(css).not.toMatch(/\.ledgerRow \{[^}]*border: 1px solid var\(--tc-border\);\s*border-radius/);
  });

  it('sets the ledger row address in the display font token, weight 600', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.ledgerRow__address \{[^}]*font-family: var\(--tc-font-display\)/);
    expect(css).toMatch(/\.ledgerRow__address \{[^}]*font-weight: 600/);
  });

  it('sets the ledger row meta column (ref + date) in the mono font role', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.ledgerRow__meta \{[^}]*font-family: var\(--tc-font-mono\)/);
  });

  it('styles the whole-row link to inherit colour with no underline', () => {
    const css = pageStyles();
    expect(css).toContain('.ledgerRow__link');
    expect(css).toMatch(/\.ledgerRow__link \{[^}]*color: inherit/);
    expect(css).toMatch(/\.ledgerRow__link \{[^}]*text-decoration: none/);
  });

  it('styles the "View details" affordance in the amber accent colour', () => {
    const css = pageStyles();
    expect(css).toContain('.ledgerRow__cta');
    expect(css).toMatch(/\.ledgerRow__cta \{[^}]*var\(--tc-amber\)/);
  });

  it('styles the stamp outlined (currentColor border, no fill) with an uppercase letterspaced label', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.stamp \{[^}]*border: 1\.5px solid currentColor/);
    expect(css).toMatch(/\.stamp \{[^}]*background: transparent/);
    expect(css).toMatch(/\.stamp__label \{[^}]*text-transform: uppercase/);
    expect(css).toMatch(/\.stamp__label \{[^}]*letter-spacing: 0\.12em/);
  });

  it('keeps the three canonical stamp colour modifiers', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.stamp--granted \{[^}]*color: var\(--tc-status-granted\)/);
    expect(css).toMatch(/\.stamp--refused \{[^}]*color: var\(--tc-status-refused\)/);
    expect(css).toMatch(/\.stamp--neutral \{[^}]*color: var\(--tc-status-neutral\)/);
  });
});

describe('pageStyles townLinks', () => {
  it('styles the .townLinks__list and its anchors with design tokens', () => {
    const css = pageStyles();
    expect(css).toContain('.townLinks__list');
    expect(css).toContain('.townLinks__list a');
    // Uses design tokens (var(--tc-*)), never hard-coded colours/spacing.
    expect(css).toMatch(/\.townLinks__list a \{[^}]*var\(--tc-/);
  });
});

describe('pageStyles status colour tokens (decision 4: shared palette, light-first)', () => {
  /**
   * @param {string} css
   * @returns {string} the declarations inside the top-level `:root { ... }`
   *   block (i.e. NOT the one nested inside a @media query).
   */
  function rootDeclarations(css) {
    const match = css.match(/^\s*:root \{([^}]*)\}/m);
    return match ? match[1] : '';
  }

  /**
   * @param {string} css
   * @returns {string} the declarations inside `:root { ... }` nested within
   *   `@media (prefers-color-scheme: dark)`.
   */
  function darkMediaDeclarations(css) {
    const match = css.match(
      /@media \(prefers-color-scheme: dark\) \{\s*:root \{([^}]*)\}/,
    );
    return match ? match[1] : '';
  }

  it('defines the three canonical status colour tokens, light-first', () => {
    const root = rootDeclarations(pageStyles());
    expect(root).toMatch(/--tc-status-granted: #1A7D37;/);
    expect(root).toMatch(/--tc-status-refused: #C42B2B;/);
    expect(root).toMatch(/--tc-status-neutral: #6D665C;/);
  });

  it('moves the dark variants of the status tokens into the dark media query', () => {
    const dark = darkMediaDeclarations(pageStyles());
    expect(dark).toMatch(/--tc-status-granted: #34C759;/);
    expect(dark).toMatch(/--tc-status-refused: #FF453A;/);
    expect(dark).toMatch(/--tc-status-neutral: #A69E8F;/);
  });

  it('no longer defines the old ad-hoc per-state status tokens', () => {
    const css = pageStyles();
    expect(css).not.toContain('--tc-status-permitted');
    expect(css).not.toContain('--tc-status-conditions');
    expect(css).not.toContain('--tc-status-rejected');
    expect(css).not.toContain('--tc-status-withdrawn');
    expect(css).not.toContain('--tc-status-appealed');
    expect(css).not.toContain('--tc-status-default');
  });
});

describe('pageStyles light-first token flip (tc-r4n9.1)', () => {
  /**
   * @param {string} css
   * @returns {string} the declarations inside the top-level `:root { ... }`
   *   block (i.e. NOT the one nested inside a @media query).
   */
  function rootDeclarations(css) {
    const match = css.match(/^\s*:root \{([^}]*)\}/m);
    return match ? match[1] : '';
  }

  /**
   * @param {string} css
   * @returns {string} the declarations inside `:root { ... }` nested within
   *   `@media (prefers-color-scheme: dark)`.
   */
  function darkMediaDeclarations(css) {
    const match = css.match(
      /@media \(prefers-color-scheme: dark\) \{\s*:root \{([^}]*)\}/,
    );
    return match ? match[1] : '';
  }

  it('defaults :root to the light-mode values directly (no query needed)', () => {
    const root = rootDeclarations(pageStyles());
    expect(root).toMatch(/--tc-amber: #9E6709;/);
    expect(root).toMatch(/--tc-amber-hover: #8A5F06;/);
    expect(root).toMatch(/--tc-background: #F5F0E6;/);
    expect(root).toMatch(/--tc-surface: #FFFDF6;/);
    expect(root).toMatch(/--tc-text-primary: #24201A;/);
    expect(root).toMatch(/--tc-text-secondary: #6D665C;/);
    expect(root).toMatch(/--tc-text-on-accent: #FFFDF8;/);
    expect(root).toMatch(/--tc-border: #DAD2C2;/);
  });

  it('moves the former dark defaults, byte-for-byte, into @media (prefers-color-scheme: dark)', () => {
    const dark = darkMediaDeclarations(pageStyles());
    expect(dark).toMatch(/--tc-amber: #E9A620;/);
    expect(dark).toMatch(/--tc-amber-hover: #F0B83A;/);
    expect(dark).toMatch(/--tc-background: #191713;/);
    expect(dark).toMatch(/--tc-surface: #23201A;/);
    expect(dark).toMatch(/--tc-text-primary: #EFE9DC;/);
    expect(dark).toMatch(/--tc-text-secondary: #A69E8F;/);
    expect(dark).toMatch(/--tc-text-on-accent: #1C1917;/);
    expect(dark).toMatch(/--tc-border: #3A352B;/);
  });

  it('no longer gates light values behind a prefers-color-scheme: light query', () => {
    expect(pageStyles()).not.toContain('prefers-color-scheme: light');
  });

  it('keeps exactly one dark override block, gated on prefers-color-scheme: dark', () => {
    const css = pageStyles();
    const matches = css.match(/@media \(prefers-color-scheme: dark\)/g) ?? [];
    expect(matches).toHaveLength(1);
  });

  it('records the chosen background scale as a comment in the token block, converged with the share page family', () => {
    const css = pageStyles();
    const root = rootDeclarations(css);
    // The comment documenting WHY these values were chosen must live inside the
    // :root token block itself (not just JSDoc above the function), so it is
    // visible to anyone reading the generated page source.
    expect(root).toMatch(/\/\*[^]*share page[^]*\*\//i);
    // Value-agnostic on purpose: the comment references the token names, not the
    // hexes, so a palette flip (e.g. Public Notice v2) can't leave it stale.
    expect(root.toLowerCase()).toContain('background scale');
    expect(root).toContain('--tc-background');
  });
});

describe('renderMidListCta and mid-list injection (tc-fgoyj, filed-notice CTA band)', () => {
  const STORE_HREF = 'https://apps.apple.com/x?pt=1&ct=seo-lpa-mid&mt=8';
  const MID_CARD = 'class="ledgerCta"';

  /** @param {number} count */
  function manyApplications(count) {
    return Array.from({ length: count }, (_, i) =>
      application({
        uid: `BDB/2026/${1000 + i}`,
        name: `26/${1000 + i}/FUL`,
        address: `${i + 1} Mill Road, Basingstoke, RG21 1AA`,
      }),
    );
  }

  it('renders a filed-notice card CTA naming the area, never disguised as an application', () => {
    const html = renderMidListCta('Basingstoke and Deane', STORE_HREF);
    expect(html).toContain(MID_CARD);
    expect(html).toContain('Get told when the next one lands');
    expect(html).toContain('Town Crier watches Basingstoke and Deane');
    // The store href is a build-time literal, interpolated as-is (escaping
    // would mangle the & in its query string).
    expect(html).toContain(`href="${STORE_HREF}"`);
    expect(html).toContain('rel="noopener"');
    expect(html).toContain('target="_blank"');
    // A pitch card must not carry ledger-row furniture.
    expect(html).not.toContain('ledgerRow__address');
    expect(html).not.toContain('View details');
  });

  it('HTML-escapes the area name', () => {
    const html = renderMidListCta('<script>alert(1)</script>', STORE_HREF);
    expect(html).not.toContain('<script>alert(1)</script>');
    expect(html).toContain('&lt;script&gt;');
  });

  it('injects the card after the 8th application when the list has 12 or more', () => {
    const html = renderApplicationsList(manyApplications(12), SLUG, {
      area: 'Basingstoke and Deane',
      storeHref: STORE_HREF,
    });
    expect(html).toContain(MID_CARD);
    expect(html.indexOf('8 Mill Road')).toBeLessThan(html.indexOf(MID_CARD));
    expect(html.indexOf(MID_CARD)).toBeLessThan(html.indexOf('9 Mill Road'));
  });

  it('skips the card below 12 applications, and always without a midCta argument', () => {
    const withCta = renderApplicationsList(manyApplications(11), SLUG, {
      area: 'Basingstoke and Deane',
      storeHref: STORE_HREF,
    });
    expect(withCta).not.toContain(MID_CARD);
    const withoutArg = renderApplicationsList(manyApplications(12), SLUG);
    expect(withoutArg).not.toContain(MID_CARD);
  });
});

describe('pageStyles CTA bands (filed-notice card shape, 2px brass top rule, display-font heading)', () => {
  it('gives the mid-list CTA card a 2px brass top rule and a display-font heading', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.ledgerCta \{[^}]*border-top: 2px solid var\(--tc-amber\)/);
    expect(css).toMatch(/\.ledgerCta__heading \{[^}]*font-family: var\(--tc-font-display\)/);
  });

  it('gives the bottom CTA banner the same 2px brass top rule and display-font heading treatment', () => {
    const css = pageStyles();
    expect(css).toMatch(/\.cta \{[^}]*border-top: 2px solid var\(--tc-amber\)/);
    expect(css).toMatch(/\.cta__heading \{[^}]*font-family: var\(--tc-font-display\)/);
  });
});

describe('renderQrBlock (tc-fgoyj)', () => {
  const STORE_HREF = 'https://apps.apple.com/x?pt=1&ct=seo-lpa-qr&mt=8';

  it('wraps an inline QR SVG and a plain-language caption', () => {
    const html = renderQrBlock(STORE_HREF);
    expect(html).toContain('class="cta__qr"');
    expect(html).toContain('<svg class="qr" role="img"');
    expect(html).toContain('aria-label="QR code linking to Town Crier on the App Store"');
    expect(html).toContain('Or scan with your phone camera to get the app.');
  });

  it('hides the block on touch devices and shows it for mouse/trackpad pointers', () => {
    const css = pageStyles();
    expect(css).toContain('.cta__qr { display: none; }');
    const mediaBlock = css.slice(css.indexOf('@media (hover: hover) and (pointer: fine)'));
    expect(mediaBlock).toContain('.cta__qr {');
    expect(mediaBlock).toContain('display: flex;');
  });
});
