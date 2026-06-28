/**
 * Static Web Apps redirect-config merge for the same-name SEO dedup (tc-77ll /
 * #717). Suppressed `/planning/<x>/<x>` town URLs are in the sitemap and likely
 * indexed; simply not emitting their HTML leaves them to the SWA
 * `navigationFallback` (a 200 + SPA shell = soft-404, worse than the duplicate).
 * Instead the prerender folds a permanent (301) redirect to the owning authority
 * page into the hand-written base `staticwebapp.config.json`.
 *
 * This module is pure: it takes the parsed base config plus the set of
 * suppressed town paths and returns a new merged config. The prerender reads the
 * base from `web/public/staticwebapp.config.json`, calls this, and writes the
 * result into the build's outDir (`web/dist`).
 *
 * Route-count headroom: SWA allows far more routes than we generate. Even at the
 * full gazetteer there are ≤151 same-name redirects plus a handful of base
 * routes — comfortably within the SWA limit.
 */

/**
 * Merge generated 301 redirect routes for suppressed same-name town pages into a
 * base Static Web Apps config, without dropping any hand-written base route or
 * disturbing the rest of the config (navigationFallback, globalHeaders, ...).
 *
 * Each path is the suppressed town's path relative to `/planning` (e.g.
 * `"wrexham/wrexham"`); the redirect target is the authority page derived from
 * the first (authority-slug) segment. Duplicate paths collapse to one route.
 *
 * @param {{ routes?: Array<object> } & Record<string, unknown>} baseConfig
 * @param {ReadonlyArray<string>} redirectPaths suppressed town paths ("<auth>/<town>")
 * @returns {{ routes: Array<object> } & Record<string, unknown>} a new merged config
 */
export function mergeRedirects(baseConfig, redirectPaths) {
  const seen = new Set();
  /** @type {Array<object>} */
  const redirectRoutes = [];
  for (const path of redirectPaths) {
    const route = `/planning/${path}`;
    if (seen.has(route)) {
      continue;
    }
    seen.add(route);
    const authoritySlug = path.split('/')[0];
    redirectRoutes.push({
      route,
      redirect: `/planning/${authoritySlug}`,
      statusCode: 301,
    });
  }
  return {
    ...baseConfig,
    routes: [...(baseConfig.routes ?? []), ...redirectRoutes],
  };
}
