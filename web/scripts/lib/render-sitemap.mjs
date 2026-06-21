import { SITE_ORIGIN } from './constants.mjs';

/**
 * Render a sitemap.xml listing every generated planning page as an absolute
 * canonical URL under the public origin.
 *
 * @param {readonly string[]} slugs published authority slugs
 * @returns {string}
 */
export function renderSitemap(slugs) {
  const entries = slugs
    .map(
      (slug) =>
        `  <url>\n    <loc>${SITE_ORIGIN}/planning/${slug}</loc>\n  </url>`,
    )
    .join('\n');
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${entries}
</urlset>
`;
}
