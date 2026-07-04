/**
 * Canonical origin for the public marketing/SEO site. Every generated page,
 * canonical link, OG url and sitemap entry is anchored here.
 * @type {string}
 */
export const SITE_ORIGIN = 'https://towncrierapp.uk';

/**
 * Canonical origin for the public per-application share pages
 * (`/a/{authoritySlug}/{ref}`). Mirrors the iOS `ShareURL.origin`
 * (`mobile/ios/.../Sharing/ShareURL.swift`) and the API `shareOrigin`
 * (`api-go/internal/sharepage/view.go`) — the three MUST stay in lockstep.
 * @type {string}
 */
export const SHARE_ORIGIN = 'https://share.towncrierapp.uk';

/**
 * Build the canonical share URL for one planning application:
 * `https://share.towncrierapp.uk/a/{authoritySlug}/{ref}`.
 *
 * `ref` is the application's PlanIt `name` (the `planit_name` column, exposed as
 * `name` on the SEO snapshot) — the exact key the share page looks up by. It can
 * contain slashes (e.g. `Kingston/25/02755/CLC`), which are kept as path
 * separators; every other segment is percent-encoded. This mirrors the iOS
 * `ShareURL.build` behaviour (`.urlPathAllowed`, which keeps `/`). The slug is
 * already URL-safe (lowercase-hyphenated `Slugify` output), so it is used
 * verbatim. Returns `null` when either component is empty, so a caller can omit
 * the link rather than emit a broken one.
 *
 * @param {string} authoritySlug
 * @param {string} ref
 * @returns {string | null}
 */
export function shareUrl(authoritySlug, ref) {
  if (!authoritySlug || !ref) {
    return null;
  }
  const encodedRef = ref.split('/').map(encodeURIComponent).join('/');
  return `${SHARE_ORIGIN}/a/${authoritySlug}/${encodedRef}`;
}

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
 * Numeric App Store id for the iOS app — the tail of {@link APP_DOWNLOAD_URL}.
 * Sourced once here (and mirrored in `src/config/links.ts`, kept in lockstep by
 * the drift-guard test) so the Apple Smart App Banner meta tag never hardcodes
 * it per template.
 * @type {string}
 */
export const APPLE_APP_ID = '6764095657';

/**
 * Build a campaign-tagged App Store link from the campaign-free
 * {@link APP_DOWNLOAD_URL}. The `ct` token surfaces under App Store Connect →
 * App Analytics → Acquisition so installs can be attributed to the surface that
 * sent them (e.g. `seo-lpa`, `seo-town`, `web-home`); `mt=8` is Apple's
 * software-app media type. Cookieless and aggregate — sets nothing on-device.
 *
 * `APP_DOWNLOAD_URL` is deliberately left campaign-free so the byte-equal
 * lockstep guard between this file and `src/config/links.ts` still holds.
 *
 * @param {string} campaign
 * @returns {string}
 */
export function appStoreUrl(campaign) {
  return `${APP_DOWNLOAD_URL}?ct=${encodeURIComponent(campaign)}&mt=8`;
}

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
