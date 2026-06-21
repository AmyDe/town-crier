import { describe, it, expect } from 'vitest';
import { renderPlanningPage } from '../render-page.mjs';
import { APP_DOWNLOAD_URL, SITE_ORIGIN } from '../constants.mjs';

/**
 * @param {Partial<import('../render-page.mjs').PlanningPageData>} [overrides]
 * @returns {import('../render-page.mjs').PlanningPageData}
 */
function pageData(overrides = {}) {
  return {
    slug: 'basingstoke-and-deane',
    areaName: 'Basingstoke and Deane',
    authorityId: 100,
    total: 42,
    totalCapped: false,
    applications: [
      {
        uid: 'BDB/2026/0001',
        name: '26/0001/FUL',
        address: '12 Mill Road, Basingstoke, RG21 1AA',
        description: 'Erection of two-storey rear extension',
        appState: 'Permitted',
        startDate: '2026-01-15',
        link: 'https://planit.org.uk/planapplic/26-0001-FUL',
        url: 'https://www.basingstoke.gov.uk/planning/26-0001-FUL',
      },
      {
        uid: 'BDB/2026/0002',
        name: '26/0002/HSE',
        address: '5 High Street, Basingstoke, RG21 7QN',
        description: 'Single-storey side extension and new boundary wall',
        appState: 'Rejected',
        startDate: '2026-02-03',
        link: 'https://planit.org.uk/planapplic/26-0002-HSE',
        url: null,
      },
    ],
    ...overrides,
  };
}

describe('renderPlanningPage', () => {
  it('is a complete HTML document with the lang attribute', () => {
    const html = renderPlanningPage(pageData());
    expect(html.startsWith('<!doctype html>')).toBe(true);
    expect(html).toContain('<html lang="en">');
  });

  it('renders the area H1', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain(
      '<h1>Planning applications in Basingstoke and Deane</h1>',
    );
  });

  it('includes a canonical link to the towncrierapp.uk slug', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain(
      `<link rel="canonical" href="${SITE_ORIGIN}/planning/basingstoke-and-deane"`,
    );
  });

  it('includes Open Graph tags', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('property="og:title"');
    expect(html).toContain('property="og:type"');
    expect(html).toContain(
      `property="og:url" content="${SITE_ORIGIN}/planning/basingstoke-and-deane"`,
    );
  });

  it('embeds schema.org ItemList JSON-LD', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('application/ld+json');
    expect(html).toContain('"@type":"ItemList"');
  });

  it('renders each application address, status label, date and links', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('12 Mill Road, Basingstoke, RG21 1AA');
    expect(html).toContain('Erection of two-storey rear extension');
    expect(html).toContain('Granted'); // Permitted -> Granted
    expect(html).toContain('Refused'); // Rejected -> Refused
    expect(html).toContain('15 Jan 2026');
    expect(html).toContain(
      'https://www.basingstoke.gov.uk/planning/26-0001-FUL',
    );
    expect(html).toContain('https://planit.org.uk/planapplic/26-0001-FUL');
  });

  it('shows the exact total in the lead line when not capped', () => {
    const html = renderPlanningPage(pageData({ total: 42, totalCapped: false }));
    expect(html).toContain('42');
  });

  it('shows a capped count as "200+" when the read hit the cap', () => {
    const html = renderPlanningPage(
      pageData({ total: 200, totalCapped: true }),
    );
    expect(html).toContain('200+');
  });

  it('renders a stats block with counts by status', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toMatch(/Granted[\s\S]*?1/);
  });

  it('includes the evergreen how-to-comment explainer for the area', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain(
      'How to comment on a planning application in Basingstoke and Deane',
    );
  });

  it('includes a CTA to the App Store for the area', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('Get push alerts for Basingstoke and Deane');
    expect(html).toContain(APP_DOWNLOAD_URL);
  });

  it('renders an above-the-fold download button in the site header', () => {
    const html = renderPlanningPage(pageData());
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
    const html = renderPlanningPage(pageData());
    expect(html).toContain('<section class="cta">');
    expect(html).toContain('class="cta__button"');
    expect(html).toContain('Download on the App Store');
  });

  it('carries the mandatory PlanIt + OGL + OS + OSM attribution', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('PlanIt');
    expect(html).toContain('Open Government Licence');
    expect(html).toContain('Ordnance Survey');
    expect(html).toContain('OpenStreetMap');
  });

  it('escapes HTML in application fields to prevent markup injection', () => {
    const html = renderPlanningPage(
      pageData({
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
        total: 12,
      }),
    );
    expect(html).not.toContain('<script>alert(1)</script>');
    expect(html).toContain('&lt;script&gt;alert(1)&lt;/script&gt;');
  });
});
