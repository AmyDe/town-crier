import { SITE_ORIGIN } from './constants.mjs';

/**
 * @typedef {Object} SitemapEntry
 * @property {string} path        page path under /planning/, e.g. "adur" or
 *                                "cornwall/truro"
 * @property {string} [lastmod]   the page's content-derived freshness signal —
 *                                the ISO `lastDifferent` of the most-recently
 *                                changed application shown on the page. Reduced
 *                                to a W3C `YYYY-MM-DD` date in the output;
 *                                omitted from the `<url>` when absent/invalid.
 */

/**
 * Reduce an ISO `lastDifferent` timestamp to the `YYYY-MM-DD` date the sitemap
 * needs. A W3C sitemap `<lastmod>` accepts a bare calendar date, which is the
 * cleanest honest signal here (the time-of-day adds noise Google ignores).
 * Returns `undefined` for null/empty/unparseable input so the caller can omit
 * the tag rather than emit an invalid/empty one.
 *
 * @param {string | null | undefined} iso
 * @returns {string | undefined} the `YYYY-MM-DD` date, or undefined
 */
export function sitemapLastmod(iso) {
  if (iso === null || iso === undefined || iso === '') {
    return undefined;
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return date.toISOString().slice(0, 10);
}

/**
 * Render a sitemap.xml listing every generated planning page as an absolute
 * canonical URL under the public origin, each carrying a content-derived
 * `<lastmod>` (the date of the freshest application the page shows) when one is
 * known. The `<lastmod>` is deliberately NOT the build clock — bumping it on
 * every rebuild of an unchanged page teaches search engines to distrust it.
 *
 * @param {ReadonlyArray<SitemapEntry>} entries published page entries
 * @returns {string}
 */
export function renderSitemap(entries) {
  const urls = entries
    .map((entry) => {
      const loc = `    <loc>${SITE_ORIGIN}/planning/${entry.path}</loc>`;
      const lastmod = sitemapLastmod(entry.lastmod);
      const lines = lastmod
        ? `${loc}\n    <lastmod>${lastmod}</lastmod>`
        : loc;
      return `  <url>\n${lines}\n  </url>`;
    })
    .join('\n');
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls}
</urlset>
`;
}
