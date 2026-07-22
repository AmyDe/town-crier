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

  describe('breadcrumb (tc-r4n9.4, extended to include the /planning hub tc-3ht16)', () => {
    it('renders a visible breadcrumb with exactly three levels: Home, the hub, then the authority', () => {
      const html = renderPlanningPage(pageData());
      const nav = html.match(
        /<nav class="breadcrumb" aria-label="Breadcrumb">[\s\S]*?<\/nav>/,
      );
      expect(nav).not.toBeNull();
      const [navHtml] = nav;
      const liCount = (navHtml.match(/<li/g) ?? []).length;
      expect(liCount).toBe(3);
      expect(navHtml).toContain('<li><a href="/">Town Crier</a></li>');
      expect(navHtml).toContain(
        '<li><a href="/planning">Planning applications</a></li>',
      );
      expect(navHtml).toContain('<li>Basingstoke and Deane</li>');
      // The current page is plain text, not a self-link (matches the town-page pattern).
      expect(navHtml).not.toContain('<a href="/planning/basingstoke-and-deane"');
    });

    it('places the breadcrumb between the site header and the main content', () => {
      const html = renderPlanningPage(pageData());
      expect(html.indexOf('</header>')).toBeLessThan(
        html.indexOf('class="breadcrumb"'),
      );
      expect(html.indexOf('class="breadcrumb"')).toBeLessThan(
        html.indexOf('<main>'),
      );
    });

    it('HTML-escapes the authority name in the visible breadcrumb', () => {
      const html = renderPlanningPage(
        pageData({ areaName: 'Stoke & <b>Bramley</b>' }),
      );
      expect(html).not.toContain('<li><b>Bramley</b></li>');
      expect(html).toContain('<li>Stoke &amp; &lt;b&gt;Bramley&lt;/b&gt;</li>');
    });

    it('embeds a three-item BreadcrumbList in the schema.org JSON-LD, alongside the ItemList and Dataset', () => {
      const html = renderPlanningPage(pageData());
      expect(html).toContain('"@type":"BreadcrumbList"');
      const ld = html.match(
        /<script type="application\/ld\+json">([\s\S]*?)<\/script>/,
      );
      expect(ld).not.toBeNull();
      const [, json] = ld;
      const parsed = JSON.parse(json);
      const breadcrumb = parsed.find((entry) => entry['@type'] === 'BreadcrumbList');
      expect(breadcrumb).toBeDefined();
      expect(breadcrumb.itemListElement).toEqual([
        {
          '@type': 'ListItem',
          position: 1,
          name: 'Town Crier',
          item: `${SITE_ORIGIN}/`,
        },
        {
          '@type': 'ListItem',
          position: 2,
          name: 'Planning applications',
          item: `${SITE_ORIGIN}/planning`,
        },
        {
          '@type': 'ListItem',
          position: 3,
          name: 'Basingstoke and Deane',
          item: `${SITE_ORIGIN}/planning/basingstoke-and-deane`,
        },
      ]);
    });

    it('carries the same name sequence, and the same hrefs for every linked crumb, in the visible breadcrumb and the JSON-LD BreadcrumbList', () => {
      const html = renderPlanningPage(pageData());
      const navHtml = html.match(
        /<nav class="breadcrumb" aria-label="Breadcrumb">[\s\S]*?<\/nav>/,
      )[0];
      const visibleNames = [
        ...navHtml.matchAll(/<li>(?:<a[^>]*>)?([^<]+)(?:<\/a>)?<\/li>/g),
      ].map((m) => m[1]);
      const visibleLinkedHrefs = [
        ...navHtml.matchAll(/<li><a href="([^"]+)">/g),
      ].map((m) => m[1]);

      const [, json] = html.match(
        /<script type="application\/ld\+json">([\s\S]*?)<\/script>/,
      );
      const breadcrumb = JSON.parse(json).find(
        (entry) => entry['@type'] === 'BreadcrumbList',
      );
      const jsonLdNames = breadcrumb.itemListElement.map((entry) => entry.name);
      // The final crumb (the current page) is plain text in the visible nav, so
      // only the LINKED crumbs need matching hrefs.
      const jsonLdLinkedHrefs = breadcrumb.itemListElement
        .slice(0, -1)
        .map((entry) => entry.item.replace(SITE_ORIGIN, ''));

      expect(visibleNames).toEqual(jsonLdNames);
      expect(visibleLinkedHrefs).toEqual(jsonLdLinkedHrefs);
    });
  });

  describe('towns-index cross-link (tc-3ht16, GH #990 slice 1)', () => {
    it('links to /planning/towns when the authority has published towns', () => {
      const html = renderPlanningPage(
        pageData({ towns: [{ name: 'Basingstoke', slug: 'basingstoke' }] }),
      );
      expect(html).toContain('href="/planning/towns"');
    });

    it('still links to /planning/towns when the authority has zero published towns (the 29-authority case)', () => {
      const html = renderPlanningPage(pageData({ towns: [] }));
      expect(html).toContain('href="/planning/towns"');
    });

    it('also links to the /planning hub', () => {
      const html = renderPlanningPage(pageData());
      expect(html).toContain('href="/planning"');
    });
  });

  it('renders each application address as the headline and status label', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain(
      '<h3 class="ledgerRow__address">12 Mill Road, Basingstoke, RG21 1AA</h3>',
    );
    expect(html).toContain('Erection of two-storey rear extension');
    expect(html).toContain('Granted'); // Permitted -> Granted
    expect(html).toContain('Refused'); // Rejected -> Refused
  });

  it('demotes the reference to small ledger-row metadata and removes per-row external links (decisions 5 & 6)', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('<p class="ledgerRow__ref">26/0001/FUL</p>');
    expect(html).not.toContain('<h3 class="ledgerRow__ref">');
    // The per-row PlanIt/council links are gone (decision 6) — official-record
    // links now live on the share page only.
    expect(html).not.toContain(
      'https://www.basingstoke.gov.uk/planning/26-0001-FUL',
    );
    expect(html).not.toContain('https://planit.org.uk/planapplic/26-0001-FUL');
    expect(html).not.toContain('class="appLink"');
  });

  it('makes the whole row a real anchor to its share page, with a visible "View details" affordance', () => {
    const html = renderPlanningPage(pageData());
    // app.name (26/0001/FUL) is the ref, slashes kept as separators.
    expect(html).toContain(
      '<a class="ledgerRow__link" href="https://share.towncrierapp.uk/a/basingstoke-and-deane/26/0001/FUL">',
    );
    expect(html).toContain('<span class="ledgerRow__cta">View details →</span>');
    // The share page is the item URL in the ItemList structured data too.
    expect(html).toContain(
      '"url":"https://share.towncrierapp.uk/a/basingstoke-and-deane/26/0001/FUL"',
    );
  });

  it('renders the cards in the order the applications were supplied (already lastDifferent DESC upstream)', () => {
    const html = renderPlanningPage(pageData());
    expect(html.indexOf('12 Mill Road, Basingstoke, RG21 1AA')).toBeLessThan(
      html.indexOf('5 High Street, Basingstoke, RG21 7QN'),
    );
  });

  describe('single "Data updated" line (tc-r4n9.3, replacing the per-card repetition)', () => {
    it('renders exactly one "Data updated" line, near the H1, from the freshest shown application date', () => {
      const html = renderPlanningPage(pageData());
      const occurrences = (html.match(/class="dataUpdated"/g) ?? []).length;
      expect(occurrences).toBe(1);
      expect(html).toContain('<p class="dataUpdated">Data updated 12 Jun 2026</p>');
      expect(html.indexOf('<h1>')).toBeLessThan(html.indexOf('class="dataUpdated"'));
      expect(html.indexOf('class="dataUpdated"')).toBeLessThan(html.indexOf('class="lead"'));
    });

    it('no longer repeats the old "Last updated" line once per card', () => {
      const html = renderPlanningPage(pageData());
      expect(html).not.toContain('Last updated');
    });

    // tc-s0yf (GH #819) deliberately reintroduces a per-row date line — under a
    // NEW class (`ledgerRow__date`) and format (Started/Decided, sourced from
    // the application's own real-world dates, not a re-index marker) — distinct
    // from the old "Last updated" line this describe block's title refers to.
    it('renders the Started/Decided date line once per row (tc-s0yf)', () => {
      const html = renderPlanningPage(pageData());
      expect(html).toContain(
        '<p class="ledgerRow__date">Started 15 Jan 2026 · Awaiting decision</p>',
      );
      expect(html).toContain(
        '<p class="ledgerRow__date">Started 3 Feb 2026 · Awaiting decision</p>',
      );
    });
  });

  it('shows the exact total in the lead line', () => {
    const html = renderPlanningPage(pageData({ total: 42 }));
    // Apostrophe in "what's" is HTML-escaped by the shared escapeHtml() call.
    expect(html).toContain(
      "See what&#39;s happening with planning in Basingstoke and Deane: 42 planning applications tracked so far.",
    );
  });

  it('renders a compact status summary from the server breakdown, not the visible cards', () => {
    const html = renderPlanningPage(pageData());
    // 30 Granted comes from the server breakdown over the bounded read; only two
    // cards are rendered, so a count of 30 proves the summary is server-driven.
    expect(html).toContain('<h2 class="statusSummary__heading">Status breakdown</h2>');
    expect(html).toMatch(/30[\s\S]{0,20}Granted/);
    expect(html).toMatch(/8[\s\S]{0,20}Refused/);
    expect(html).toMatch(/4[\s\S]{0,20}Undecided/);
    expect(html).toMatch(/42[\s\S]{0,20}total/);
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

  it('tags each download CTA with its own per-surface campaign token (tc-fgoyj)', () => {
    const html = renderPlanningPage(pageData());
    // One ct token per placement so App Analytics attributes store arrivals
    // to the surface that sent them. The QR block has no href (its link lives
    // in the QR modules) and the mid-list card needs a longer list, so the
    // 2-app fixture carries exactly these three.
    for (const surface of ['seo-lpa-hdr', 'seo-lpa-inline', 'seo-lpa-btm']) {
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
        uid: `BDB/2026/${1000 + i}`,
        name: `26/${1000 + i}/FUL`,
        address: `${i + 1} Mill Road, Basingstoke, RG21 1AA`,
        description: 'Erection of two-storey rear extension',
        appState: 'Permitted',
        startDate: '2026-01-15',
        lastDifferent: '2026-06-12T09:30:00+00:00',
        link: null,
        url: null,
      }));
    }

    // The bare class name also appears in the inline stylesheet, so assertions
    // target the rendered card markup.
    const MID_CARD = 'class="ledgerCta"';

    it('slots a CTA card into a long list, after the eighth application', () => {
      const html = renderPlanningPage(pageData({ applications: manyApplications(12) }));
      expect(html).toContain(MID_CARD);
      expect(html).toContain(`href="${appStoreUrl('seo-lpa-mid')}"`);
      expect(html).toContain('Get told when the next one lands');
      // After the 8th card, before the 9th. Scoped to the rendered list —
      // the JSON-LD in <head> repeats the addresses much earlier in the page.
      const list = html.slice(html.indexOf('<ul class="ledger">'));
      expect(list.indexOf('8 Mill Road')).toBeLessThan(list.indexOf(MID_CARD));
      expect(list.indexOf(MID_CARD)).toBeLessThan(list.indexOf('9 Mill Road'));
    });

    it('omits the card on a short list so two CTAs never sit almost back to back', () => {
      const html = renderPlanningPage(pageData());
      expect(html).not.toContain(MID_CARD);
      expect(html).not.toContain('ct=seo-lpa-mid');
    });
  });

  describe('desktop QR block (tc-fgoyj)', () => {
    it('renders an inline QR code inside the bottom CTA banner', () => {
      const html = renderPlanningPage(pageData());
      const cta = html.match(/<section class="cta">[\s\S]*?<\/section>/);
      expect(cta).not.toBeNull();
      const [ctaHtml] = cta;
      expect(ctaHtml).toContain('class="cta__qr"');
      expect(ctaHtml).toContain('<svg class="qr" role="img"');
      expect(ctaHtml).toContain('scan with your phone camera');
    });

    it('hides the QR block on touch devices via the pointer media query', () => {
      const html = renderPlanningPage(pageData());
      expect(html).toContain('.cta__qr { display: none; }');
      expect(html).toContain('@media (hover: hover) and (pointer: fine)');
    });
  });

  it('includes a CTA to the App Store for the area', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('Get push alerts for Basingstoke and Deane');
    expect(html).toContain(APP_DOWNLOAD_URL);
  });

  describe('inline alerts CTA above the list (tc-r4n9.3)', () => {
    it('renders an inline "Get push alerts" CTA directly after the intro, above the applications list', () => {
      const html = renderPlanningPage(pageData());
      expect(html).toContain('class="ctaInline"');
      // Directly after the lead paragraph...
      expect(html.indexOf('class="lead"')).toBeLessThan(html.indexOf('class="ctaInline"'));
      // ...and above the applications list.
      expect(html.indexOf('class="ctaInline"')).toBeLessThan(html.indexOf('class="ledger"'));
    });

    it('keeps the existing bottom banner CTA in addition to the inline one (not a replacement)', () => {
      const html = renderPlanningPage(pageData());
      expect(html).toContain('<section class="cta">');
      expect(html.indexOf('class="ledger"')).toBeLessThan(html.indexOf('<section class="cta">'));
    });
  });

  it('renders an above-the-fold download button in the site header', () => {
    const html = renderPlanningPage(pageData());
    const header = html.match(/<header class="siteHeader">[\s\S]*?<\/header>/);
    expect(header).not.toBeNull();
    const [headerHtml] = header;
    expect(headerHtml).toContain('siteHeader__cta');
    expect(headerHtml).toContain(`href="${appStoreUrl('seo-lpa-hdr')}"`);
    expect(headerHtml).toContain('Get the app');
    // Match the bottom CTA's link safety exactly.
    expect(headerHtml).toContain('rel="noopener"');
    expect(headerHtml).toContain('target="_blank"');
    // The brand link stays on the left.
    expect(headerHtml).toContain('>Town Crier</a>');
  });

  describe('masthead (double rule, small-caps wordmark)', () => {
    it('renders the double rule beneath the masthead, inside the site header', () => {
      const html = renderPlanningPage(pageData());
      const header = html.match(/<header class="siteHeader">[\s\S]*?<\/header>/);
      expect(header).not.toBeNull();
      const [headerHtml] = header;
      expect(headerHtml).toContain('class="siteHeader__ruleHeavy"');
      expect(headerHtml).toContain('class="siteHeader__ruleHairline"');
    });

    it('gives the wordmark its own small-caps class', () => {
      const html = renderPlanningPage(pageData());
      expect(html).toContain('<a href="/" class="siteHeader__wordmark">Town Crier</a>');
    });
  });

  it('opens the applications ledger with a small-caps brass section label', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('<h2 class="ledger__heading">Latest notices</h2>');
  });

  it('keeps the rich bottom CTA block when the header CTA is present', () => {
    const html = renderPlanningPage(pageData());
    expect(html).toContain('<section class="cta">');
    expect(html).toContain('class="cta__button"');
    // "free" is honest (the download is free; the free tier is the weekly
    // digest) and one of the oldest CTR levers there is.
    expect(html).toContain('Download free on the App Store');
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
      expect(html.indexOf('class="ledger"')).toBeLessThan(
        html.indexOf('class="townLinks"'),
      );
      expect(html.indexOf('class="townLinks"')).toBeLessThan(
        html.indexOf('class="explainer"'),
      );
    });

    it('omits the section entirely when there are no published towns', () => {
      // The stylesheet always carries the .townLinks rules, so assert on the
      // section element, not the bare substring.
      expect(renderPlanningPage(pageData({ towns: [] }))).not.toContain(
        '<section class="townLinks">',
      );
      // Undefined towns (the default) also omit the section — backwards compatible.
      expect(renderPlanningPage(pageData())).not.toContain(
        '<section class="townLinks">',
      );
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
