import { describe, it, expect } from 'vitest';
import { renderSitemap, sitemapLastmod } from '../render-sitemap.mjs';
import { SITE_ORIGIN } from '../constants.mjs';

describe('renderSitemap', () => {
  it('opens with the XML declaration and urlset envelope', () => {
    const xml = renderSitemap([{ path: 'adur' }]);
    expect(xml.startsWith('<?xml version="1.0" encoding="UTF-8"?>')).toBe(true);
    expect(xml).toContain(
      '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    );
    expect(xml.trimEnd().endsWith('</urlset>')).toBe(true);
  });

  it('emits one absolute canonical loc per entry path', () => {
    const xml = renderSitemap([
      { path: 'adur' },
      { path: 'basingstoke-and-deane' },
    ]);
    expect(xml).toContain(`<loc>${SITE_ORIGIN}/planning/adur</loc>`);
    expect(xml).toContain(
      `<loc>${SITE_ORIGIN}/planning/basingstoke-and-deane</loc>`,
    );
    const urlCount = (xml.match(/<url>/g) ?? []).length;
    expect(urlCount).toBe(2);
  });

  it('emits a content-derived <lastmod> as a W3C YYYY-MM-DD date', () => {
    const xml = renderSitemap([
      { path: 'adur', lastmod: '2026-06-15T10:00:00+00:00' },
    ]);
    // Only the date portion of the ISO lastDifferent — the cleanest valid
    // W3C sitemap date.
    expect(xml).toContain('<lastmod>2026-06-15</lastmod>');
    // Never the full ISO datetime.
    expect(xml).not.toContain('<lastmod>2026-06-15T10:00:00+00:00</lastmod>');
  });

  it('omits <lastmod> entirely for an entry with no lastmod', () => {
    const xml = renderSitemap([{ path: 'adur' }]);
    expect(xml).toContain(`<loc>${SITE_ORIGIN}/planning/adur</loc>`);
    expect(xml).not.toContain('<lastmod>');
  });

  it('omits <lastmod> when the lastmod is an empty/invalid string rather than emitting an empty tag', () => {
    const xml = renderSitemap([
      { path: 'adur', lastmod: '' },
      { path: 'truro', lastmod: 'not-a-date' },
    ]);
    expect(xml).not.toContain('<lastmod>');
    expect(xml).not.toContain('<lastmod></lastmod>');
  });

  it('emits lastmod per-url independently — some dated, some not', () => {
    const xml = renderSitemap([
      { path: 'adur', lastmod: '2026-06-15T10:00:00+00:00' },
      { path: 'truro' },
    ]);
    expect((xml.match(/<lastmod>/g) ?? []).length).toBe(1);
    expect(xml).toContain('<lastmod>2026-06-15</lastmod>');
  });

  it('produces a valid empty urlset for zero entries', () => {
    const xml = renderSitemap([]);
    expect(xml).toContain('<urlset');
    expect((xml.match(/<url>/g) ?? []).length).toBe(0);
  });

  // tc-geq7h.1 (GH #821 Phase 1): the /planning/ hub itself is a sitemap entry
  // with an empty path — its canonical is exactly /planning, with no trailing
  // slash, matching the <link rel="canonical"> the hub page renders.
  it('renders an empty path as the bare /planning root, with no trailing slash', () => {
    const xml = renderSitemap([{ path: '' }]);
    expect(xml).toContain(`<loc>${SITE_ORIGIN}/planning</loc>`);
    expect(xml).not.toContain(`<loc>${SITE_ORIGIN}/planning/</loc>`);
  });

  it('supports a lastmod on the root /planning entry same as any other', () => {
    const xml = renderSitemap([
      { path: '', lastmod: '2026-06-20T10:00:00+00:00' },
    ]);
    expect(xml).toContain(`<loc>${SITE_ORIGIN}/planning</loc>`);
    expect(xml).toContain('<lastmod>2026-06-20</lastmod>');
  });
});

describe('sitemapLastmod', () => {
  it('reduces an ISO datetime to its YYYY-MM-DD date portion', () => {
    expect(sitemapLastmod('2026-06-15T10:00:00+00:00')).toBe('2026-06-15');
  });

  it('returns undefined for null, empty, or unparseable input', () => {
    expect(sitemapLastmod(null)).toBeUndefined();
    expect(sitemapLastmod(undefined)).toBeUndefined();
    expect(sitemapLastmod('')).toBeUndefined();
    expect(sitemapLastmod('not-a-date')).toBeUndefined();
  });
});
