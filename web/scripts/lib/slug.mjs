/**
 * Convert a PlanIt authority name into a lowercase-hyphenated URL slug.
 *
 *   "Basingstoke and Deane"  -> "basingstoke-and-deane"
 *   "Bristol, City of"       -> "bristol-city-of"
 *   "King's Lynn and ..."    -> "kings-lynn-and-..."
 *   "Hammersmith & Fulham"   -> "hammersmith-and-fulham"
 *
 * Ampersands expand to "and"; apostrophes are stripped (so they do not become
 * hyphens); every other run of non-alphanumeric characters collapses to a
 * single hyphen, with leading/trailing hyphens trimmed.
 *
 * @param {string} name
 * @returns {string}
 */
export function slugify(name) {
  return name
    .toLowerCase()
    .replace(/&/g, ' and ')
    .replace(/['’]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}
