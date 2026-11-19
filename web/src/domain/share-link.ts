const SHARE_ORIGIN = 'https://share.towncrierapp.uk';

/**
 * Builds the public share-page URL for a search result. `reference` is
 * deliberately not URL-encoded: PlanIt references contain slashes that the
 * share page's trailing-wildcard route `/a/{authoritySlug}/{ref...}` must see
 * as literal path segments.
 */
export function buildShareUrl(authoritySlug: string, reference: string): string {
  return `${SHARE_ORIGIN}/a/${authoritySlug}/${reference}`;
}
