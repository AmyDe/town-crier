/**
 * Canonical origin for the public marketing/SEO site. Every generated page,
 * canonical link, OG url and sitemap entry is anchored here.
 * @type {string}
 */
export const SITE_ORIGIN = 'https://towncrierapp.uk';

/**
 * Public App Store listing for the iOS app. Mirrors `src/config/links.ts`
 * (`APP_DOWNLOAD_URL`) — kept in lockstep by the drift-guard test in
 * `src/__tests__/planning-cta-link.test.ts`, because this build-time `.mjs`
 * script cannot import the app's TypeScript modules at runtime.
 * @type {string}
 */
export const APP_DOWNLOAD_URL =
  'https://apps.apple.com/gb/app/town-crier-planning-alerts/id6764095657';

/**
 * Mandatory data attribution lines (ADR 0006). Byte-for-byte the same copy the
 * app shows on its Settings → Attribution panel. Required on every public page.
 * @type {readonly string[]}
 */
export const ATTRIBUTION_LINES = [
  'Planning data provided by PlanIt (planit.org.uk)',
  'Contains public sector information licensed under the Open Government Licence. Crown Copyright.',
  'Contains Ordnance Survey data © Crown Copyright and database right.',
  'Map data © OpenStreetMap contributors.',
];

/**
 * Attribution lines for TOWN pages only. Town pages are geolocated from the ONS
 * Built-Up-Areas (2022) centroid gazetteer and (for Scotland) the NRS settlement
 * gazetteer — both Open Government Licence sources that the authority pages do
 * NOT use. So rather than widening the shared {@link ATTRIBUTION_LINES} (which
 * would over-credit authority pages with sources they don't draw on), town pages
 * extend the base list with these two extra credits. Both are OGL, mirroring the
 * existing Crown Copyright line.
 * @type {readonly string[]}
 */
export const TOWN_ATTRIBUTION_LINES = [
  ...ATTRIBUTION_LINES,
  'Town locations contain Built-Up Areas (2022) data from the Office for National Statistics, licensed under the Open Government Licence. Crown Copyright.',
  'Scottish town locations contain data from National Records of Scotland, licensed under the Open Government Licence. Crown Copyright.',
];
