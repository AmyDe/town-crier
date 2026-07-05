import { describe, it, expect } from 'vitest';
import {
  TOWNS_INDEX_SLUG,
  assertNoTownsSlugCollision,
  groupTownIndexEntries,
  renderTownsIndexPage,
} from '../render-towns-index.mjs';
import { SITE_ORIGIN, APPLE_APP_ID, APP_DOWNLOAD_URL, appStoreUrl } from '../constants.mjs';

/**
 * @param {Partial<import('../render-towns-index.mjs').TownIndexEntry>} [overrides]
 * @returns {import('../render-towns-index.mjs').TownIndexEntry}
 */
function entry(overrides = {}) {
  return {
    townName: 'Truro',
    townSlug: 'truro',
    authoritySlug: 'cornwall',
    authorityName: 'Cornwall',
    ...overrides,
  };
}

describe('TOWNS_INDEX_SLUG', () => {
  it('is the reserved "towns" path segment', () => {
    expect(TOWNS_INDEX_SLUG).toBe('towns');
  });
});

describe('assertNoTownsSlugCollision', () => {
  it('does not throw when no authority slugifies to "towns"', () => {
    expect(() =>
      assertNoTownsSlugCollision([
        { name: 'Cornwall' },
        { name: 'Basingstoke and Deane' },
      ]),
    ).not.toThrow();
  });

  it('throws loudly when an authority name slugifies to "towns"', () => {
    expect(() =>
      assertNoTownsSlugCollision([{ name: 'Cornwall' }, { name: 'Towns' }]),
    ).toThrow(/towns/i);
  });

  it('throws when an authority name normalizes (via slugify) to "towns", not just an exact-string match', () => {
    // slugify lowercases and strips punctuation, so "Towns!" would still collide.
    expect(() =>
      assertNoTownsSlugCollision([{ name: 'Towns!' }]),
    ).toThrow();
  });

  it('does not throw for an empty authority list', () => {
    expect(() => assertNoTownsSlugCollision([])).not.toThrow();
  });
});

describe('groupTownIndexEntries', () => {
  it('groups entries into A-Z sections by the town name\'s first letter', () => {
    const sections = groupTownIndexEntries([
      entry({ townName: 'Truro', townSlug: 'truro' }),
      entry({ townName: 'Adur', townSlug: 'adur' }),
      entry({ townName: 'Aberdeen', townSlug: 'aberdeen' }),
    ]);

    expect(sections.map((s) => s.letter)).toEqual(['A', 'T']);
    expect(sections[0].entries.map((e) => e.townName)).toEqual([
      'Aberdeen',
      'Adur',
    ]);
    expect(sections[1].entries.map((e) => e.townName)).toEqual(['Truro']);
  });

  it('sorts sections A-Z and entries within a section alphabetically', () => {
    const sections = groupTownIndexEntries([
      entry({ townName: 'Zennor', townSlug: 'zennor' }),
      entry({ townName: 'Bath', townSlug: 'bath' }),
    ]);
    expect(sections.map((s) => s.letter)).toEqual(['B', 'Z']);
  });

  it('breaks ties between same-named towns by authority name', () => {
    const sections = groupTownIndexEntries([
      entry({
        townName: 'Richmond',
        townSlug: 'richmond',
        authoritySlug: 'north-yorkshire',
        authorityName: 'North Yorkshire',
      }),
      entry({
        townName: 'Richmond',
        townSlug: 'richmond',
        authoritySlug: 'richmond-upon-thames',
        authorityName: 'Richmond upon Thames',
      }),
    ]);
    expect(sections).toHaveLength(1);
    expect(sections[0].entries.map((e) => e.authorityName)).toEqual([
      'North Yorkshire',
      'Richmond upon Thames',
    ]);
  });

  it('buckets a town name that does not start with A-Z under "#", sorted last', () => {
    const sections = groupTownIndexEntries([
      entry({ townName: '1610 New Town', townSlug: 'new-town' }),
      entry({ townName: 'Zennor', townSlug: 'zennor' }),
    ]);
    expect(sections.map((s) => s.letter)).toEqual(['Z', '#']);
  });

  it('returns an empty array for zero entries', () => {
    expect(groupTownIndexEntries([])).toEqual([]);
  });
});

describe('renderTownsIndexPage', () => {
  it('is a complete HTML document with the lang attribute', () => {
    const html = renderTownsIndexPage([entry()]);
    expect(html.startsWith('<!doctype html>')).toBe(true);
    expect(html).toContain('<html lang="en">');
  });

  it('renders the H1 and canonicalises to /planning/towns', () => {
    const html = renderTownsIndexPage([entry()]);
    expect(html).toContain('<h1>Planning applications by town</h1>');
    expect(html).toContain(
      `<link rel="canonical" href="${SITE_ORIGIN}/planning/towns"`,
    );
    expect(html).toContain(
      `property="og:url" content="${SITE_ORIGIN}/planning/towns"`,
    );
  });

  it('links each town to /planning/<authority-slug>/<town-slug> with the authority name as visible context', () => {
    const html = renderTownsIndexPage([
      entry({
        townName: 'Truro',
        townSlug: 'truro',
        authoritySlug: 'cornwall',
        authorityName: 'Cornwall',
      }),
    ]);
    expect(html).toContain('<a href="/planning/cornwall/truro">Truro</a>');
    expect(html).toMatch(
      /<a href="\/planning\/cornwall\/truro">Truro<\/a>[\s\S]{0,80}Cornwall/,
    );
  });

  it('renders an A-Z section per letter, each with its own heading', () => {
    const html = renderTownsIndexPage([
      entry({ townName: 'Adur', townSlug: 'adur' }),
      entry({ townName: 'Truro', townSlug: 'truro' }),
    ]);
    expect(html).toContain('id="letter-A"');
    expect(html).toContain('id="letter-T"');
    expect(html).toMatch(/<h2[^>]*>A<\/h2>/);
    expect(html).toMatch(/<h2[^>]*>T<\/h2>/);
  });

  it('renders towns in the same order groupTownIndexEntries would produce', () => {
    const html = renderTownsIndexPage([
      entry({ townName: 'Truro', townSlug: 'truro' }),
      entry({ townName: 'Adur', townSlug: 'adur' }),
    ]);
    expect(html.indexOf('>Adur<')).toBeLessThan(html.indexOf('id="letter-T"'));
  });

  it('HTML-escapes town and authority names', () => {
    const html = renderTownsIndexPage([
      entry({
        townName: 'St <Ives>',
        townSlug: 'st-ives',
        authorityName: 'Corn & Wall',
      }),
    ]);
    expect(html).toContain('St &lt;Ives&gt;');
    expect(html).toContain('Corn &amp; Wall');
    expect(html).not.toContain('St <Ives>');
  });

  it('renders a message and no A-Z sections when there are zero towns', () => {
    const html = renderTownsIndexPage([]);
    // The stylesheet always carries the .townsIndex__section rules, so assert
    // on the actual section element, not the bare class-name substring.
    expect(html).not.toContain('<section class="townsIndex__section"');
    expect(html.toLowerCase()).toContain('no town');
  });

  it('embeds a BreadcrumbList schema.org script', () => {
    const html = renderTownsIndexPage([entry()]);
    expect(html).toContain('application/ld+json');
    expect(html).toContain('"@type":"BreadcrumbList"');
  });

  it('emits the Apple Smart App Banner meta tag', () => {
    const html = renderTownsIndexPage([entry()]);
    expect(html).toContain(
      `<meta name="apple-itunes-app" content="app-id=${APPLE_APP_ID}" />`,
    );
  });

  it('tags CTAs with the ct=seo-towns-index campaign token', () => {
    const html = renderTownsIndexPage([entry()]);
    const tagged = appStoreUrl('seo-towns-index');
    expect(html).toContain(`href="${tagged}"`);
    expect(html).not.toContain(`href="${APP_DOWNLOAD_URL}"`);
  });

  it('includes the mandatory PlanIt/OGL data attribution', () => {
    const html = renderTownsIndexPage([entry()]);
    expect(html).toContain('PlanIt');
    expect(html).toContain('Open Government Licence');
  });

  it('includes the count of towns in the lead line', () => {
    const html = renderTownsIndexPage([entry(), entry({ townName: 'Adur', townSlug: 'adur' })]);
    expect(html).toContain('2 towns');
  });
});
