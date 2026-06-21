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
    totalCapped: false,
    applications: [
      {
        uid: 'CW/2026/0001',
        name: '26/0001',
        address: 'Lemon Quay, Truro, TR1 2LW',
        description: 'Change of use of ground floor from retail to café',
        appState: 'Permitted',
        startDate: '2026-01-12',
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

  it('renders each application address, status label, date and links', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('Lemon Quay, Truro, TR1 2LW');
    expect(html).toContain('Change of use of ground floor from retail to café');
    expect(html).toContain('Granted'); // Permitted -> Granted
    expect(html).toContain('Refused'); // Rejected -> Refused
    expect(html).toContain('12 Jan 2026');
    expect(html).toContain('https://planning.cornwall.gov.uk/26-0001');
    expect(html).toContain('https://planit.org.uk/planapplic/CW-26-0001');
  });

  it('shows the exact total in the lead line when not capped', () => {
    const html = renderTownPage(townData({ total: 18, totalCapped: false }));
    expect(html).toContain('18');
    expect(html).toContain('Truro');
  });

  it('shows a capped count as "200+" when the read hit the cap', () => {
    const html = renderTownPage(townData({ total: 200, totalCapped: true }));
    expect(html).toContain('200+');
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

  it('carries the mandatory PlanIt + OGL + OS + OSM attribution', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('PlanIt');
    expect(html).toContain('Open Government Licence');
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
