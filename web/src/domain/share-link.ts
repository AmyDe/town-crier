const SHARE_ORIGIN = 'https://share.towncrierapp.uk';

/**
 * Builds the public share-page URL for a search result (#821 Phase 4). The
 * result's `reference` is already the correct path segment — the anonymous
 * search API returns `planit_name`, not `uid`, specifically so this link
 * resolves (see api-go SearchResult doc comment).
 *
 * `reference` is concatenated raw, deliberately NOT URL-encoded: a PlanIt
 * reference such as "9/P/2026/0044/HH" legitimately contains slashes, and the
 * share page's route (`GET /a/{authoritySlug}/{ref...}`) matches them as
 * literal path segments via a trailing wildcard
 * (api-go/internal/sharepage/handler.go) — encoding them would turn one
 * wildcard segment into a literal "%2F" that fails to resolve.
 */
export function buildShareUrl(authoritySlug: string, reference: string): string {
  return `${SHARE_ORIGIN}/a/${authoritySlug}/${reference}`;
}
