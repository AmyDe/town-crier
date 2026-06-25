/**
 * Public download link for the iOS app.
 *
 * Points at the live App Store listing. Every download CTA reads from here,
 * so swapping this single value updates them all.
 */
export const APP_DOWNLOAD_URL =
  'https://apps.apple.com/gb/app/town-crier-planning-alerts/id6764095657';

/**
 * Numeric App Store id for the iOS app — the tail of {@link APP_DOWNLOAD_URL}.
 * Mirrors `scripts/lib/constants.mjs` (the `.mjs` prerender scripts cannot import
 * this TS at runtime); the two are kept in lockstep by the drift-guard test in
 * `src/__tests__/planning-cta-link.test.ts`. Feeds the Apple Smart App Banner
 * meta tag in `index.html` so the id is never hardcoded there.
 */
export const APPLE_APP_ID = '6764095657';

/**
 * Build a campaign-tagged App Store link from the campaign-free
 * {@link APP_DOWNLOAD_URL}. The `ct` token lets App Store Connect attribute
 * installs to the surface that sent them (e.g. `web-home` for the SPA CTAs);
 * `mt=8` is Apple's software-app media type. Cookieless and aggregate.
 *
 * `APP_DOWNLOAD_URL` stays campaign-free so the byte-equal lockstep guard with
 * `scripts/lib/constants.mjs` still holds.
 */
export function appStoreUrl(campaign: string): string {
  return `${APP_DOWNLOAD_URL}?ct=${encodeURIComponent(campaign)}&mt=8`;
}
