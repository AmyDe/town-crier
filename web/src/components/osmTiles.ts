/**
 * Shared OpenStreetMap tile constants. Hoisted out of `features/Map/MapPage.tsx`
 * (tc-rrv7i.2 / GH#863) so `features/Search`'s location-picker map can reuse the
 * same tile source and attribution without a forbidden cross-feature import —
 * both features depend on this shared module instead of on each other.
 */
export const OSM_TILE_URL = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
export const OSM_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';
