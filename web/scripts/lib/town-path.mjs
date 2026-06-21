/**
 * Town-page URL helpers. Town pages live one level below their authority page in
 * a nested hierarchy:
 *
 *   /planning/<authority-slug>              <- authority page (Phase 1)
 *   /planning/<authority-slug>/<town-slug>  <- town page (Phase 2)
 *
 * The nesting is what keeps town pages from colliding with authority pages and
 * gives crawlers a clean breadcrumb trail. The authority slug for a town is
 * resolved from its `authorityId` against the committed authority list — never
 * stored in the gazetteer, so the two can never drift.
 */

import { slugify } from './slug.mjs';

/**
 * @typedef {Object} ResolvedAuthority
 * @property {string} name display name (the PlanIt area name)
 * @property {string} slug lowercase-hyphenated URL slug
 */

/**
 * Resolve a town's `authorityId` to its parent authority's display name and URL
 * slug, using the committed authority list as the single source of truth.
 * Throws loudly when the id is absent — a malformed gazetteer must fail the
 * build, never silently emit an orphan page.
 *
 * @param {number} authorityId
 * @param {ReadonlyArray<{ id: number, name: string }>} authorities
 * @returns {ResolvedAuthority}
 */
export function resolveAuthority(authorityId, authorities) {
  const match = authorities.find((a) => a.id === authorityId);
  if (!match) {
    throw new Error(
      `no authority with id ${authorityId} in the authority list`,
    );
  }
  return { name: match.name, slug: slugify(match.name) };
}

/**
 * Build the path of a town page relative to `/planning`: the parent authority
 * slug joined to the town slug. Always two segments, so it can never equal the
 * single-segment authority page path.
 *
 * @param {string} authoritySlug
 * @param {string} townSlug
 * @returns {string} e.g. "cornwall/truro"
 */
export function townPagePath(authoritySlug, townSlug) {
  return `${authoritySlug}/${townSlug}`;
}
