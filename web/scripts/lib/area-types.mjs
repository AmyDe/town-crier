/**
 * The PlanIt `areaType` values that map 1:1 to a UK local planning authority
 * and therefore qualify for an SEO page. The excluded set (English County,
 * English Region, Metropolitan County, UK Nation, Combined Planning Authority,
 * Cross Border Area, Other Planning Entity, Crown Dependency/Dependencies) are
 * aggregates or overlap their constituent districts and would create
 * duplicate/competing pages. See issue #570 "Pre-Resolved Design Decisions".
 *
 * @type {ReadonlySet<string>}
 */
export const QUALIFYING_AREA_TYPES = new Set([
  'English District',
  'English Unitary Authority',
  'Council District',
  'Metropolitan Borough',
  'London Borough',
  'Scottish Council',
  'Welsh Principal Area',
  'National Park',
  'Northern Ireland District',
]);

/**
 * @param {string} areaType
 * @returns {boolean}
 */
export function isQualifyingAreaType(areaType) {
  return QUALIFYING_AREA_TYPES.has(areaType);
}

/**
 * @template {{ areaType: string }} T
 * @param {readonly T[]} authorities
 * @returns {T[]}
 */
export function filterQualifyingAuthorities(authorities) {
  return authorities.filter((a) => isQualifyingAreaType(a.areaType));
}
