/**
 * Same-name dedup decision for the programmatic SEO town pages (tc-77ll / #717).
 *
 * A town page at `/planning/<authority>/<town>` whose town slug equals its
 * authority's slug (e.g. `/planning/wrexham/wrexham`) duplicates the stronger
 * authority page `/planning/wrexham` with an IDENTICAL `<title>`. That is
 * keyword cannibalization: two URLs competing for the same query, splitting link
 * equity. The authority page is the broader, stronger page and should own the
 * `<name> planning` query outright, so the same-name town page is suppressed.
 *
 * The decision is made on the NORMALIZED slug, never the raw display name.
 * Strict `slugify(authorityName) === town.slug` catches every collision in the
 * current committed data (151 of them). The extra normalization below is purely
 * forward-looking: it is a NO-OP against today's plain authority names
 * ("Bristol", "Hull", "Herefordshire", "Wrexham") and only bites IF a future
 * `authorities.json` ever carries PlanIt's raw administrative/bilingual forms
 * ("Bristol, City of", "Wrexham / Wrecsam"). It is deliberately conservative so
 * it never over-matches a genuinely different nearby place — "Hull" must not
 * suppress "Kingston upon Hull", and "Herefordshire" must not suppress
 * "Hereford" (their slugs differ).
 */

import { slugify } from './slug.mjs';

/**
 * Reduce a PlanIt authority display name to the underlying place name so it is
 * comparable to a town slug, stripping forward-looking administrative/bilingual
 * decorations. Applied in addition to (never instead of) the strict slug match.
 *
 *   "Bristol, City of"      -> "Bristol"
 *   "Herefordshire, County of" -> "Herefordshire"
 *   "Wrexham / Wrecsam"     -> "Wrexham"
 *   "Wrexham (Wrecsam)"     -> "Wrexham"
 *   "Wrexham"               -> "Wrexham"  (no-op)
 *
 * @param {string} name
 * @returns {string}
 */
function canonicalAuthorityName(name) {
  let n = name;
  // Bilingual slash variant ("Anglesey / Ynys Môn") — keep the first segment.
  const slash = n.indexOf('/');
  if (slash !== -1) {
    n = n.slice(0, slash);
  }
  // Trailing parenthetical ("Wrexham (Wrecsam)") — drop it. Disambiguating
  // parentheticals like "New Forest (District)" collapse to "New Forest" too,
  // which is harmless: no town in such an authority is slugged with the bare
  // base name, so nothing is wrongly suppressed.
  n = n.replace(/\s*\([^)]*\)\s*$/, '');
  // Administrative suffix ("Bristol, City of" / "Herefordshire, County of").
  n = n.replace(/,\s*(?:city|county)\s+of\s*$/i, '');
  return n.trim();
}

/**
 * Decide whether a town is the same place as its parent authority — i.e. its
 * page would cannibalize the authority page — and so should be suppressed.
 *
 * @param {string} authorityName  the authority's display name (PlanIt area name)
 * @param {string} townSlug       the town's URL slug from the gazetteer
 * @returns {boolean} true when the town duplicates its authority page
 */
export function isSameNameAsAuthority(authorityName, townSlug) {
  // Strict equality first — this alone catches every current collision.
  if (slugify(authorityName) === townSlug) {
    return true;
  }
  // Forward-looking: a normalized administrative/bilingual form may still denote
  // the same place even when the raw name's slug differs.
  return slugify(canonicalAuthorityName(authorityName)) === townSlug;
}
