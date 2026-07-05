import type { SearchResult } from '../types';

/**
 * Response shape of one anonymous search call. `refineQuery` is true when the
 * match set exceeded the server's limit — the caller should nudge the user to
 * narrow their search rather than treat `results` as exhaustive.
 */
export interface SearchOutcome {
  readonly results: readonly SearchResult[];
  readonly refineQuery: boolean;
}

/**
 * Anonymous application search (#821 Phase 3/4) —
 * `GET /v1/applications/search`. Public and unauthenticated: no auth token is
 * ever attached to this call, so it must keep working for a fully logged-out
 * visitor.
 */
export interface SearchPort {
  /**
   * @param query non-empty search text (reference, address fragment, or
   *   description keywords).
   * @param authority optional authority id-or-slug filter; `null` searches
   *   across all authorities.
   */
  search(query: string, authority: string | null): Promise<SearchOutcome>;
}
