import { describe, it, expect } from 'vitest';
import { renderTownPage } from '../render-town-page.mjs';
import { APP_DOWNLOAD_URL, SITE_ORIGIN } from '../constants.mjs';

/**
 * @param {Partial<import('../render-town-page.mjs').TownPageData>} [overrides]
 * @returns {import('../render-town-page.mjs').TownPageData}
 */
function townData(overrides = {}) {
  return {
    townName: 'Truro',
    townSlug: 'truro',
    authorityName: 'Cornwall',
    authoritySlug: 'cornwall',
    authorityId: 52,
    total: 18,
    // Server-provided distribution over the bounded read: deliberately sums to
    // more than the two cards below, proving the stats use the breakdown.
    statusBreakdown: [
      { appState: 'Permitted', count: 12 },
      { appState: 'Rejected', count: 4 },
      { appState: null, count: 2 },
    ],
    applications: [
      {
        uid: 'CW/2026/0001',
        name: '26/0001',
        address: 'Lemon Quay, Truro, TR1 2LW',
        description: 'Change of use of ground floor from retail to café',
        appState: 'Permitted',
        startDate: '2026-01-12',
        lastDifferent: '2026-06-12T09:30:00+00:00',
        link: 'https://planit.org.uk/planapplic/CW-26-0001',
        url: 'https://planning.cornwall.gov.uk/26-0001',
      },
      {
        uid: 'CW/2026/0002',
        name: '26/0002',
        address: 'Boscawen Street, Truro, TR1 2QU',
        description: 'Two-storey rear extension to a listed building',
        appState: 'Rejected',
        startDate: '2026-02-01',
        lastDifferent: '2026-06-10T08:00:00+00:00',
        link: 'https://planit.org.uk/planapplic/CW-26-0002',
        url: null,
      },
    ],
    ...overrides,
  };
}

describe('renderTownPage', () => {
  it('is a complete HTML document with the lang attribute', () => {
    const html = renderTownPage(townData());
    expect(html.startsWith('<!doctype html>')).toBe(true);
    expect(html).toContain('<html lang="en">');
  });

  it('renders the town H1 (not the authority)', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('<h1>Planning applications in Truro</h1>');
  });

  it('canonicalises to the NESTED /planning/<authority>/<town> URL', () => {
    const html = renderTownPage(townData());
    expect(html).toContain(
      `<link rel="canonical" href="${SITE_ORIGIN}/planning/cornwall/truro"`,
    );
  });

  it('sets the OG url to the nested town URL', () => {
    const html = renderTownPage(townData());
    expect(html).toContain(
      `property="og:url" content="${SITE_ORIGIN}/planning/cornwall/truro"`,
    );
    expect(html).toContain('property="og:title"');
    expect(html).toContain('property="og:type"');
  });

  it('embeds schema.org ItemList and BreadcrumbList JSON-LD', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('application/ld+json');
    expect(html).toContain('"@type":"ItemList"');
    expect(html).toContain('"@type":"BreadcrumbList"');
  });

  it('renders a breadcrumb linking up to the authority page', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('class="breadcrumb"');
    expect(html).toContain(`href="${SITE_ORIGIN}/planning/cornwall"`);
    // The breadcrumb names the parent authority.
    expect(html).toMatch(/breadcrumb[\s\S]*?Cornwall/);
  });

  it('renders each application address, status label, Last updated date and links', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('Lemon Quay, Truro, TR1 2LW');
    expect(html).toContain('Change of use of ground floor from retail to café');
    expect(html).toContain('Granted'); // Permitted -> Granted
    expect(html).toContain('Refused'); // Rejected -> Refused
    // The visible card date is the lastDifferent date, labelled "Last updated".
    expect(html).toContain('Last updated 12 Jun 2026');
    expect(html).toContain('https://planning.cornwall.gov.uk/26-0001');
    expect(html).toContain('https://planit.org.uk/planapplic/CW-26-0001');
  });

  it('orders the visible Last updated dates to match the lastDifferent DESC sort', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('Last updated 12 Jun 2026');
    expect(html).toContain('Last updated 10 Jun 2026');
    expect(html.indexOf('Last updated 12 Jun 2026')).toBeLessThan(
      html.indexOf('Last updated 10 Jun 2026'),
    );
  });

  it('shows the exact total in the lead line', () => {
    const html = renderTownPage(townData({ total: 18 }));
    expect(html).toContain(
      'Town Crier is tracking 18 planning applications in Truro.',
    );
  });

  it('renders a stats block from the server breakdown, not the visible cards', () => {
    const html = renderTownPage(townData());
    // 12 Granted comes from the server breakdown over the bounded read; only two
    // cards are rendered, so a count of 12 proves the stats are server-driven.
    expect(html).toMatch(/Granted[\s\S]*?12/);
    expect(html).toMatch(/Refused[\s\S]*?4/);
    expect(html).toMatch(/Unknown[\s\S]*?2/);
  });

  it('includes the evergreen how-to-comment explainer naming the town and its authority', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('How to comment on a planning application in Truro');
    expect(html).toContain(
      'Cornwall is the local planning authority responsible',
    );
  });

  it('includes a CTA to the App Store for the town', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('Get push alerts for Truro');
    expect(html).toContain(APP_DOWNLOAD_URL);
  });

  it('renders an above-the-fold download button in the site header', () => {
    const html = renderTownPage(townData());
    const header = html.match(/<header class="siteHeader">[\s\S]*?<\/header>/);
    expect(header).not.toBeNull();
    const [headerHtml] = header;
    expect(headerHtml).toContain('siteHeader__cta');
    expect(headerHtml).toContain(`href="${APP_DOWNLOAD_URL}"`);
    expect(headerHtml).toContain('Get the app');
    // Match the bottom CTA's link safety exactly.
    expect(headerHtml).toContain('rel="noopener"');
    expect(headerHtml).toContain('target="_blank"');
    // The brand link stays on the left.
    expect(headerHtml).toContain('>Town Crier</a>');
  });

  it('keeps the rich bottom CTA block when the header CTA is present', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('<section class="cta">');
    expect(html).toContain('class="cta__button"');
    expect(html).toContain('Download on the App Store');
  });

  it('carries the mandatory PlanIt + OGL + OS + OSM attribution', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('PlanIt');
    expect(html).toContain('Open Government Licence');
    expect(html).toContain('Ordnance Survey');
    expect(html).toContain('OpenStreetMap');
  });

  it('additionally credits the ONS Built-Up-Area and NRS Scotland gazetteers (the town centroid sources) under OGL', () => {
    const html = renderTownPage(townData());
    // Town positions come from the ONS Built-Up-Areas (2022) centroid gazetteer
    // and (for Scotland) NRS settlements — both Open Government Licence sources —
    // so town pages must credit them alongside the existing lines.
    expect(html).toContain('Office for National Statistics');
    expect(html).toContain('Built-Up Areas');
    expect(html).toContain('National Records of Scotland');
    // The four base lines are still present (not replaced).
    expect(html).toContain('PlanIt');
    expect(html).toContain('Ordnance Survey');
    expect(html).toContain('OpenStreetMap');
  });

  it('escapes HTML in application fields to prevent markup injection', () => {
    const html = renderTownPage(
      townData({
        applications: [
          {
            uid: 'x',
            name: 'x',
            address: '<script>alert(1)</script>',
            description: 'safe',
            appState: 'Permitted',
            startDate: null,
            lastDifferent: '2026-06-12T09:30:00+00:00',
            link: null,
            url: null,
          },
        ],
      }),
    );
    expect(html).not.toContain('<script>alert(1)</script>');
    expect(html).toContain('&lt;script&gt;alert(1)&lt;/script&gt;');
  });
});
