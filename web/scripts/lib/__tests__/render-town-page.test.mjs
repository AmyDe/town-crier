import { describe, it, expect } from 'vitest';
import { renderTownPage } from '../render-town-page.mjs';
import {
  APP_DOWNLOAD_URL,
  APPLE_APP_ID,
  SITE_ORIGIN,
  appStoreUrl,
} from '../constants.mjs';

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

  it('renders each application address as the headline and status label', () => {
    const html = renderTownPage(townData());
    expect(html).toContain(
      '<h3 class="appCard__address">Lemon Quay, Truro, TR1 2LW</h3>',
    );
    expect(html).toContain('Change of use of ground floor from retail to café');
    expect(html).toContain('Granted'); // Permitted -> Granted
    expect(html).toContain('Refused'); // Rejected -> Refused
  });

  it('demotes the reference to small card metadata and removes per-card external links (decisions 5 & 6)', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('<p class="appCard__ref">26/0001</p>');
    expect(html).not.toContain('<h3 class="appCard__ref">');
    expect(html).not.toContain('https://planning.cornwall.gov.uk/26-0001');
    expect(html).not.toContain('https://planit.org.uk/planapplic/CW-26-0001');
    expect(html).not.toContain('class="appLink"');
  });

  it('makes the whole card a real anchor to its share page, with a visible "View details" affordance', () => {
    const html = renderTownPage(townData());
    // Town-page apps are scoped to the town's own authority, so authoritySlug is
    // correct for every card.
    expect(html).toContain(
      '<a class="appCard__link" href="https://share.towncrierapp.uk/a/cornwall/26/0001">',
    );
    expect(html).toContain('<span class="appCard__cta">View details →</span>');
    expect(html).toContain(
      '"url":"https://share.towncrierapp.uk/a/cornwall/26/0001"',
    );
  });

  it('renders the cards in the order the applications were supplied (already lastDifferent DESC upstream)', () => {
    const html = renderTownPage(townData());
    expect(html.indexOf('Lemon Quay, Truro, TR1 2LW')).toBeLessThan(
      html.indexOf('Boscawen Street, Truro, TR1 2QU'),
    );
  });

  describe('single "Data updated" line (tc-r4n9.3, replacing the per-card repetition)', () => {
    it('renders exactly one "Data updated" line, near the H1, from the freshest shown application date', () => {
      const html = renderTownPage(townData());
      const occurrences = (html.match(/class="dataUpdated"/g) ?? []).length;
      expect(occurrences).toBe(1);
      expect(html).toContain('<p class="dataUpdated">Data updated 12 Jun 2026</p>');
      expect(html.indexOf('<h1>')).toBeLessThan(html.indexOf('class="dataUpdated"'));
      expect(html.indexOf('class="dataUpdated"')).toBeLessThan(html.indexOf('class="lead"'));
    });

    it('no longer repeats the old "Last updated" line once per card', () => {
      const html = renderTownPage(townData());
      expect(html).not.toContain('Last updated');
    });

    // tc-s0yf (GH #819) deliberately reintroduces a per-card date line — under a
    // NEW class (`appCard__dates`) and format (Started/Decided, sourced from the
    // application's own real-world dates, not a re-index marker) — distinct from
    // the old "Last updated" line this describe block's title refers to.
    it('renders the Started/Decided date line once per card (tc-s0yf)', () => {
      const html = renderTownPage(townData());
      expect(html).toContain(
        '<p class="appCard__dates">Started 12 Jan 2026 · Awaiting decision</p>',
      );
      expect(html).toContain(
        '<p class="appCard__dates">Started 1 Feb 2026 · Awaiting decision</p>',
      );
    });
  });

  it('shows the exact total in the lead line', () => {
    const html = renderTownPage(townData({ total: 18 }));
    // Apostrophe in "what's" is HTML-escaped by the shared escapeHtml() call.
    expect(html).toContain(
      "See what&#39;s happening with planning in Truro: 18 planning applications tracked so far.",
    );
  });

  it('renders a compact status summary from the server breakdown, not the visible cards', () => {
    const html = renderTownPage(townData());
    // 12 Granted comes from the server breakdown over the bounded read; only two
    // cards are rendered, so a count of 12 proves the summary is server-driven.
    expect(html).toContain('<h2 class="statusSummary__heading">Status breakdown</h2>');
    expect(html).toMatch(/12[\s\S]{0,20}Granted/);
    expect(html).toMatch(/4[\s\S]{0,20}Refused/);
    expect(html).toMatch(/2[\s\S]{0,20}Undecided/);
    expect(html).toMatch(/18[\s\S]{0,20}total/);
  });

  it('includes the evergreen how-to-comment explainer naming the town and its authority', () => {
    const html = renderTownPage(townData());
    expect(html).toContain('How to comment on a planning application in Truro');
    expect(html).toContain(
      'Cornwall is the local planning authority responsible',
    );
  });

  it('emits the Apple Smart App Banner meta tag in <head> after the viewport meta', () => {
    const html = renderTownPage(townData());
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

  it('tags each download CTA with its own per-surface campaign token (tc-fgoyj)', () => {
    const html = renderTownPage(townData());
    // One ct token per placement so App Analytics attributes store arrivals
    // to the surface that sent them. The QR block has no href (its link lives
    // in the QR modules) and the mid-list card needs a longer list, so the
    // 2-app fixture carries exactly these three.
    for (const surface of ['seo-town-hdr', 'seo-town-inline', 'seo-town-btm']) {
      const occurrences = html.split(`href="${appStoreUrl(surface)}"`).length - 1;
      expect(occurrences, surface).toBe(1);
    }
    // Every store link carries the provider token — Apple ignores ct without pt.
    expect(html).toContain('pt=');
    // Never the bare, campaign-free URL in a CTA href.
    expect(html).not.toContain(`href="${APP_DOWNLOAD_URL}"`);
  });

  describe('mid-list CTA card (tc-fgoyj)', () => {
    /** @returns {import('../render-shared.mjs').PlanningApplicationItem[]} */
    function manyApplications(count) {
      return Array.from({ length: count }, (_, i) => ({
        uid: `CW/2026/${1000 + i}`,
        name: `26/${1000 + i}`,
        address: `${i + 1} Lemon Quay, Truro, TR1 2LW`,
        description: 'Change of use of ground floor from retail to café',
        appState: 'Permitted',
        startDate: '2026-01-12',
        lastDifferent: '2026-06-12T09:30:00+00:00',
        link: null,
        url: null,
      }));
    }

    // The bare class name also appears in the inline stylesheet, so assertions
    // target the rendered card markup.
    const MID_CARD = 'class="appCard appCard--cta"';

    it('slots a CTA card naming the town into a long list, after the eighth application', () => {
      const html = renderTownPage(townData({ applications: manyApplications(12) }));
      expect(html).toContain(MID_CARD);
      expect(html).toContain(`href="${appStoreUrl('seo-town-mid')}"`);
      expect(html).toContain('Town Crier watches Truro');
      // After the 8th card, before the 9th. Scoped to the rendered list —
      // the JSON-LD in <head> repeats the addresses much earlier in the page.
      const list = html.slice(html.indexOf('<ul class="appList">'));
      expect(list.indexOf('8 Lemon Quay')).toBeLessThan(list.indexOf(MID_CARD));
      expect(list.indexOf(MID_CARD)).toBeLessThan(list.indexOf('9 Lemon Quay'));
    });

    it('omits the card on a short list so two CTAs never sit almost back to back', () => {
      const html = renderTownPage(townData());
      expect(html).not.toContain(MID_CARD);
      expect(html).not.toContain('ct=seo-town-mid');
    });
  });

  describe('desktop QR block (tc-fgoyj)', () => {
    it('renders an inline QR code inside the bottom CTA banner', () => {
      const html = renderTownPage(townData());
      const cta = html.match(/<section class="cta">[\s\S]*?<\/section>/);
      expect(cta).not.toBeNull();
      const [ctaHtml] = cta;
      expect(ctaHtml).toContain('class="cta__qr"');
      expect(ctaHtml).toContain('<svg class="qr" role="img"');
      expect(ctaHtml).toContain('scan with your phone camera');
    });

    it('hides the QR block on touch devices via the pointer media query', () => {
      const html = renderTownPage(townData());
      expect(html).toContain('.cta__qr { display: none; }');
      expect(html).toContain('@media (hover: hover) and (pointer: fine)');
    });
  });

  describe('inline alerts CTA above the list (tc-r4n9.3)', () => {
    it('renders an inline "Get push alerts" CTA directly after the intro, above the applications list', () => {
      const html = renderTownPage(townData());
      expect(html).toContain('class="ctaInline"');
      expect(html.indexOf('class="lead"')).toBeLessThan(html.indexOf('class="ctaInline"'));
      expect(html.indexOf('class="ctaInline"')).toBeLessThan(html.indexOf('class="appList"'));
    });

    it('keeps the existing bottom banner CTA in addition to the inline one (not a replacement)', () => {
      const html = renderTownPage(townData());
      expect(html).toContain('<section class="cta">');
      expect(html.indexOf('class="appList"')).toBeLessThan(html.indexOf('<section class="cta">'));
    });
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
    expect(headerHtml).toContain(`href="${appStoreUrl('seo-town-hdr')}"`);
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
    // "free" is honest (the download is free; the free tier is the weekly
    // digest) and one of the oldest CTR levers there is.
    expect(html).toContain('Download free on the App Store');
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
