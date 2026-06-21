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
