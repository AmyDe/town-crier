import { describe, it, expect } from 'vitest';
import { renderPlanningPage } from '../render-page.mjs';
import {
  APP_DOWNLOAD_URL,
  APPLE_APP_ID,
  SITE_ORIGIN,
  appStoreUrl,
} from '../constants.mjs';

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
    // Server-provided distribution over the bounded ~200-doc read: deliberately
    // sums to more than the two cards rendered below, proving the stats block
    // uses the breakdown rather than counting the visible cards.
    statusBreakdown: [
      { appState: 'Permitted', count: 30 },
      { appState: 'Rejected', count: 8 },
      { appState: null, count: 4 },
    ],
    applications: [
      {
        uid: 'BDB/2026/0001',
        name: '26/0001/FUL',
        address: '12 Mill Road, Basingstoke, RG21 1AA',
        description: 'Erection of two-storey rear extension',
        appState: 'Permitted',
        startDate: '2026-01-15',
        lastDifferent: '2026-06-12T09:30:00+00:00',
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
        lastDifferent: '2026-06-10T08:00:00+00:00',
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

  it('renders each application address, status label, Last updated date and links', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('12 Mill Road, Basingstoke, RG21 1AA');
    expect(html).toContain('Erection of two-storey rear extension');
    expect(html).toContain('Granted'); // Permitted -> Granted
    expect(html).toContain('Refused'); // Rejected -> Refused
    // The visible card date is the lastDifferent date, labelled "Last updated".
    expect(html).toContain('Last updated 12 Jun 2026');
    expect(html).toContain(
      'https://www.basingstoke.gov.uk/planning/26-0001-FUL',
    );
    expect(html).toContain('https://planit.org.uk/planapplic/26-0001-FUL');
  });

  it('orders the visible Last updated dates to match the lastDifferent DESC sort', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('Last updated 12 Jun 2026');
    expect(html).toContain('Last updated 10 Jun 2026');
    expect(html.indexOf('Last updated 12 Jun 2026')).toBeLessThan(
      html.indexOf('Last updated 10 Jun 2026'),
    );
  });

  it('shows the exact total in the lead line', () => {
    const html = renderPlanningPage(pageData({ total: 42 }));
    expect(html).toContain(
      'Town Crier is tracking 42 planning applications in Basingstoke and Deane.',
    );
  });

  it('renders a stats block from the server breakdown, not the visible cards', () => {
    const html = renderPlanningPage(pageData());
    // 30 Granted comes from the server breakdown over the bounded read; only two
    // cards are rendered, so a count of 30 proves the stats are server-driven.
    expect(html).toMatch(/Granted[\s\S]*?30/);
    expect(html).toMatch(/Refused[\s\S]*?8/);
    expect(html).toMatch(/Unknown[\s\S]*?4/);
  });

  it('includes the evergreen how-to-comment explainer for the area', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain(
      'How to comment on a planning application in Basingstoke and Deane',
    );
  });

  it('emits the Apple Smart App Banner meta tag in <head> after the viewport meta', () => {
    const html = renderPlanningPage(pageData());
    const head = html.match(/<head>[\s\S]*?<\/head>/);
    expect(head).not.toBeNull();
    const [headHtml] = head;
    expect(headHtml).toContain(
      `<meta name="apple-itunes-app" content="app-id=${APPLE_APP_ID}" />`,
    );
    expect(headHtml).toContain('app-id=6764095657');
    // Sits immediately after the viewport meta.
    expect(headHtml.indexOf('name="viewport"')).toBeLessThan(
      headHtml.indexOf('name="apple-itunes-app"'),
    );
  });

  it('tags every download CTA with the ct=seo-lpa campaign token', () => {
    const html = renderPlanningPage(pageData());
    const tagged = appStoreUrl('seo-lpa');
    // Both CTAs (header "Get the app" + bottom "Download on the App Store").
    const occurrences = html.split(`href="${tagged}"`).length - 1;
    expect(occurrences).toBe(2);
    expect(html).toContain('ct=seo-lpa');
    // Never the bare, campaign-free URL in a CTA href.
    expect(html).not.toContain(`href="${APP_DOWNLOAD_URL}"`);
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
    expect(headerHtml).toContain(`href="${appStoreUrl('seo-lpa')}"`);
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

  it('does NOT carry the town-only ONS Built-Up-Area / NRS gazetteer credit', () => {
    // Authority pages are not positioned from the BUA/NRS centroid gazetteer, so
    // they keep the base attribution and must not pick up the town-only lines.
    const html = renderPlanningPage(pageData());
    expect(html).not.toContain('Built-Up Areas');
    expect(html).not.toContain('National Records of Scotland');
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
            lastDifferent: '2026-06-12T09:30:00+00:00',
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

  describe('town links section', () => {
    it('renders a .townLinks section when the authority has published towns', () => {
      const html = renderPlanningPage(
        pageData({
          towns: [
            { name: 'Basingstoke', slug: 'basingstoke' },
            { name: 'Tadley', slug: 'tadley' },
          ],
        }),
      );
      expect(html).toContain('<section class="townLinks">');
      expect(html).toContain(
        '<h2>Planning applications by town in Basingstoke and Deane</h2>',
      );
      expect(html).toContain('<ul class="townLinks__list">');
    });

    it('links to the canonical nested /planning/<authority>/<town> path for each town', () => {
      const html = renderPlanningPage(
        pageData({
          towns: [
            { name: 'Basingstoke', slug: 'basingstoke' },
            { name: 'Tadley', slug: 'tadley' },
          ],
        }),
      );
      expect(html).toContain(
        '<a href="/planning/basingstoke-and-deane/basingstoke">Basingstoke</a>',
      );
      expect(html).toContain(
        '<a href="/planning/basingstoke-and-deane/tadley">Tadley</a>',
      );
    });

    it('places the town-links section immediately after the Recent applications list', () => {
      const html = renderPlanningPage(
        pageData({ towns: [{ name: 'Basingstoke', slug: 'basingstoke' }] }),
      );
      // After the recent-applications <ul>, before the how-to-comment explainer.
      expect(html.indexOf('class="appList"')).toBeLessThan(
        html.indexOf('class="townLinks"'),
      );
      expect(html.indexOf('class="townLinks"')).toBeLessThan(
        html.indexOf('class="explainer"'),
      );
    });

    it('omits the section entirely when there are no published towns', () => {
      expect(renderPlanningPage(pageData({ towns: [] }))).not.toContain(
        'townLinks',
      );
      // Undefined towns (the default) also omit the section — backwards compatible.
      expect(renderPlanningPage(pageData())).not.toContain('townLinks');
    });

    it('HTML-escapes town names in the link list', () => {
      const html = renderPlanningPage(
        pageData({
          towns: [{ name: 'Stoke & <b>Bramley</b>', slug: 'stoke-bramley' }],
        }),
      );
      expect(html).not.toContain('<b>Bramley</b>');
      expect(html).toContain('Stoke &amp; &lt;b&gt;Bramley&lt;/b&gt;');
    });
  });
});
