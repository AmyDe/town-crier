import { describe, it, expect } from 'vitest';
import { renderPlanningIndexPage } from '../render-planning-index.mjs';
import {
  APP_DOWNLOAD_URL,
  APPLE_APP_ID,
  SITE_ORIGIN,
  appStoreUrl,
} from '../constants.mjs';

/**
 * @param {Partial<import('../render-planning-index.mjs').PlanningIndexData>} [overrides]
 * @returns {import('../render-planning-index.mjs').PlanningIndexData}
 */
function indexData(overrides = {}) {
  return {
    authorities: [
      { name: 'Adur', slug: 'adur', applicationCount: 42, townCount: 2 },
      { name: 'Aberdeen', slug: 'aberdeen', applicationCount: 12, townCount: 0 },
      {
        name: 'Basingstoke and Deane',
        slug: 'basingstoke-and-deane',
        applicationCount: 1,
        townCount: 1,
      },
    ],
    ...overrides,
  };
}

describe('renderPlanningIndexPage', () => {
  it('is a complete HTML document with the lang attribute', () => {
    const html = renderPlanningIndexPage(indexData());
    expect(html.startsWith('<!doctype html>')).toBe(true);
    expect(html).toContain('<html lang="en">');
  });

  it('renders the hub H1', () => {
    const html = renderPlanningIndexPage(indexData());
    expect(html).toContain('<h1>Planning applications by council</h1>');
  });

  it('uses plural "authorities" wording in the lead and meta description for more than one authority', () => {
    const html = renderPlanningIndexPage(indexData());
    expect(html).toContain(
      '<p class="lead">Browse recent planning applications for 3 local planning authorities across the UK.</p>',
    );
    expect(html).toContain('for 3 local planning authorities across the UK');
  });

  it('uses singular "authority" wording when there is exactly one', () => {
    const html = renderPlanningIndexPage(
      indexData({
        authorities: [{ name: 'Adur', slug: 'adur', applicationCount: 42, townCount: 0 }],
      }),
    );
    expect(html).toContain(
      '<p class="lead">Browse recent planning applications for 1 local planning authority across the UK.</p>',
    );
    expect(html).not.toContain('1 local planning authorities');
  });

  it('includes a canonical link to /planning with no trailing slug or slash', () => {
    const html = renderPlanningIndexPage(indexData());
    expect(html).toContain(`<link rel="canonical" href="${SITE_ORIGIN}/planning" />`);
  });

  it('includes Open Graph tags pointing at the canonical url', () => {
    const html = renderPlanningIndexPage(indexData());
    expect(html).toContain('property="og:title"');
    expect(html).toContain('property="og:type"');
    expect(html).toContain(`property="og:url" content="${SITE_ORIGIN}/planning"`);
  });

  it('embeds an ItemList JSON-LD with one entry per authority, in order', () => {
    const html = renderPlanningIndexPage(indexData());
    const ld = html.match(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/);
    expect(ld).not.toBeNull();
    const [, json] = ld;
    const parsed = JSON.parse(json);
    const itemList = parsed.find((entry) => entry['@type'] === 'ItemList');
    expect(itemList).toBeDefined();
    expect(itemList.numberOfItems).toBe(3);
    expect(itemList.itemListElement).toEqual([
      { '@type': 'ListItem', position: 1, name: 'Adur', url: `${SITE_ORIGIN}/planning/adur` },
      {
        '@type': 'ListItem',
        position: 2,
        name: 'Aberdeen',
        url: `${SITE_ORIGIN}/planning/aberdeen`,
      },
      {
        '@type': 'ListItem',
        position: 3,
        name: 'Basingstoke and Deane',
        url: `${SITE_ORIGIN}/planning/basingstoke-and-deane`,
      },
    ]);
  });

  it('embeds a two-item BreadcrumbList: Home, then this page (no self-link)', () => {
    const html = renderPlanningIndexPage(indexData());
    const nav = html.match(/<nav class="breadcrumb" aria-label="Breadcrumb">[\s\S]*?<\/nav>/);
    expect(nav).not.toBeNull();
    const [navHtml] = nav;
    const liCount = (navHtml.match(/<li/g) ?? []).length;
    expect(liCount).toBe(2);
    expect(navHtml).toContain('<li><a href="/">Town Crier</a></li>');
    expect(navHtml).toContain('<li>Planning applications</li>');
    expect(navHtml).not.toContain('<a href="/planning"');

    const ld = html.match(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/);
    const parsed = JSON.parse(ld[1]);
    const breadcrumb = parsed.find((entry) => entry['@type'] === 'BreadcrumbList');
    expect(breadcrumb.itemListElement).toEqual([
      { '@type': 'ListItem', position: 1, name: 'Town Crier', item: `${SITE_ORIGIN}/` },
      {
        '@type': 'ListItem',
        position: 2,
        name: 'Planning applications',
        item: `${SITE_ORIGIN}/planning`,
      },
    ]);
  });

  describe('towns-index cross-link (tc-3ht16, GH #990 slice 1)', () => {
    it('links to /planning/towns', () => {
      const html = renderPlanningIndexPage(indexData());
      expect(html).toContain('href="/planning/towns"');
    });

    it('places the cross-link near the lead paragraph, above the A-Z letter sections', () => {
      const html = renderPlanningIndexPage(indexData());
      expect(html.indexOf('class="lead"')).toBeLessThan(
        html.indexOf('href="/planning/towns"'),
      );
      expect(html.indexOf('href="/planning/towns"')).toBeLessThan(
        html.indexOf('class="hubGroup"'),
      );
    });
  });

  describe('A-Z grouping', () => {
    it('groups authorities under their first-letter heading', () => {
      const html = renderPlanningIndexPage(indexData());
      expect(html).toContain('<section class="hubGroup" id="letter-A"');
      expect(html).toContain('<h2 id="letter-A-heading">A</h2>');
      expect(html).toContain('<section class="hubGroup" id="letter-B"');
      expect(html).toContain('<h2 id="letter-B-heading">B</h2>');
    });

    it('keeps entries within the same letter group in the order supplied (sorting happens upstream)', () => {
      const html = renderPlanningIndexPage(indexData());
      const groupA = html.match(
        /<section class="hubGroup" id="letter-A"[\s\S]*?<\/section>/,
      )[0];
      // Adur before Aberdeen — exactly the input order, proving this module does
      // not itself re-sort (the caller sorts before handing entries in).
      expect(groupA.indexOf('>Adur<')).toBeLessThan(groupA.indexOf('>Aberdeen<'));
    });

    it('only emits a letter heading/jump-link for letters that actually have entries', () => {
      const html = renderPlanningIndexPage(indexData());
      expect(html).not.toContain('id="letter-C"');
      expect(html).not.toContain('href="#letter-C"');
      expect(html).toContain('href="#letter-A"');
      expect(html).toContain('href="#letter-B"');
    });

    it('renders zero letter sections and no crash for an empty authority list', () => {
      const html = renderPlanningIndexPage(indexData({ authorities: [] }));
      expect(html).not.toContain('<section class="hubGroup"');
      expect(html.startsWith('<!doctype html>')).toBe(true);
    });
  });

  describe('authority entries', () => {
    it('links each entry to its /planning/<slug> page', () => {
      const html = renderPlanningIndexPage(indexData());
      expect(html).toContain('<a class="hubList__link" href="/planning/adur">Adur</a>');
      expect(html).toContain(
        '<a class="hubList__link" href="/planning/basingstoke-and-deane">Basingstoke and Deane</a>',
      );
    });

    it('shows the application count and town count', () => {
      const html = renderPlanningIndexPage(indexData());
      expect(html).toContain('42 applications tracked · 2 towns');
    });

    it('uses singular "application"/"town" wording for a count of exactly 1', () => {
      const html = renderPlanningIndexPage(indexData());
      expect(html).toContain('1 application tracked · 1 town');
    });

    it('omits the town count entirely when an authority has zero published towns', () => {
      const html = renderPlanningIndexPage(indexData());
      expect(html).toContain('12 applications tracked</span>');
      expect(html).not.toContain('12 applications tracked · 0 towns');
    });

    it('HTML-escapes authority names', () => {
      const html = renderPlanningIndexPage(
        indexData({
          authorities: [
            {
              name: 'Stoke & <b>Bramley</b>',
              slug: 'stoke-bramley',
              applicationCount: 5,
              townCount: 0,
            },
          ],
        }),
      );
      expect(html).not.toContain('<b>Bramley</b>');
      expect(html).toContain('Stoke &amp; &lt;b&gt;Bramley&lt;/b&gt;');
    });
  });

  it('tags every download CTA with the ct=seo-hub campaign token', () => {
    const html = renderPlanningIndexPage(indexData());
    const tagged = appStoreUrl('seo-hub');
    const occurrences = html.split(`href="${tagged}"`).length - 1;
    expect(occurrences).toBe(3);
    expect(html).toContain('ct=seo-hub');
    expect(html).not.toContain(`href="${APP_DOWNLOAD_URL}"`);
  });

  it('emits the Apple Smart App Banner meta tag in <head>', () => {
    const html = renderPlanningIndexPage(indexData());
    const head = html.match(/<head>[\s\S]*?<\/head>/);
    expect(head).not.toBeNull();
    expect(head[0]).toContain(
      `<meta name="apple-itunes-app" content="app-id=${APPLE_APP_ID}" />`,
    );
  });

  it('carries the mandatory PlanIt + OGL + OS + OSM attribution', () => {
    const html = renderPlanningIndexPage(indexData());
    expect(html).toContain('PlanIt');
    expect(html).toContain('Open Government Licence');
    expect(html).toContain('Ordnance Survey');
    expect(html).toContain('OpenStreetMap');
  });

  describe('masthead (double rule, small-caps wordmark) — Public Notice broadsheet, consistent with the authority/town pages', () => {
    it('renders the double rule beneath the masthead, inside the site header', () => {
      const html = renderPlanningIndexPage(indexData());
      const header = html.match(/<header class="siteHeader">[\s\S]*?<\/header>/);
      expect(header).not.toBeNull();
      const [headerHtml] = header;
      expect(headerHtml).toContain('class="siteHeader__ruleHeavy"');
      expect(headerHtml).toContain('class="siteHeader__ruleHairline"');
    });

    it('gives the wordmark its own small-caps class and keeps the brass header CTA', () => {
      const html = renderPlanningIndexPage(indexData());
      expect(html).toContain('<a href="/" class="siteHeader__wordmark">Town Crier</a>');
      expect(html).toContain('siteHeader__cta');
    });
  });
});
