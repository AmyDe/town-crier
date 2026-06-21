import { describe, it, expect } from 'vitest';
import { renderSitemap } from '../render-sitemap.mjs';
import { SITE_ORIGIN } from '../constants.mjs';

describe('renderSitemap', () => {
  it('opens with the XML declaration and urlset envelope', () => {
    const xml = renderSitemap(['adur']);
    expect(xml.startsWith('<?xml version="1.0" encoding="UTF-8"?>')).toBe(true);
    expect(xml).toContain(
      '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    );
    expect(xml.trimEnd().endsWith('</urlset>')).toBe(true);
  });

  it('emits one absolute canonical loc per slug', () => {
    const xml = renderSitemap(['adur', 'basingstoke-and-deane']);
    expect(xml).toContain(`<loc>${SITE_ORIGIN}/planning/adur</loc>`);
    expect(xml).toContain(
      `<loc>${SITE_ORIGIN}/planning/basingstoke-and-deane</loc>`,
    );
    const urlCount = (xml.match(/<url>/g) ?? []).length;
    expect(urlCount).toBe(2);
  });

  it('produces a valid empty urlset for zero slugs', () => {
    const xml = renderSitemap([]);
    expect(xml).toContain('<urlset');
    expect((xml.match(/<url>/g) ?? []).length).toBe(0);
  });
});
