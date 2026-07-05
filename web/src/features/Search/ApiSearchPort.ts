import type { SearchOutcome, SearchPort } from '../../domain/ports/search-port';
import type { SearchResult } from '../../domain/types';

interface SearchResultDto {
  readonly reference: string;
  readonly authoritySlug: string;
  readonly authorityName: string;
  readonly address: string;
  readonly appState: string | null;
  readonly startDate: string | null;
  readonly decidedDate: string | null;
}

interface SearchResponseDto {
  readonly query: string;
  readonly results: readonly SearchResultDto[];
  readonly refineQuery: boolean;
}

function mapToDomain(dto: SearchResultDto): SearchResult {
  return {
    reference: dto.reference,
    authoritySlug: dto.authoritySlug,
    authorityName: dto.authorityName,
    address: dto.address,
    appState: dto.appState,
    startDate: dto.startDate,
    decidedDate: dto.decidedDate,
  };
}

/**
 * Anonymous adapter for `GET /v1/applications/search` (#821 Phase 4). Unlike
 * the authenticated `ApiClient` (`src/api/client.ts`), this never attaches an
 * `Authorization` header — the endpoint is public and must work for a fully
 * logged-out visitor, so it talks to `fetch` directly rather than going
 * through the token-bearing client (mirrors `ApiLegalDocumentPort`, the other
 * anonymous-page adapter in this codebase).
 */
export class ApiSearchPort implements SearchPort {
  private readonly baseUrl: string;
  private readonly fetchFn: typeof globalThis.fetch;

  constructor(baseUrl: string, fetchFn: typeof globalThis.fetch = globalThis.fetch.bind(globalThis)) {
    this.baseUrl = baseUrl;
    this.fetchFn = fetchFn;
  }

  async search(query: string, authority: string | null): Promise<SearchOutcome> {
    const params = new URLSearchParams({ q: query });
    if (authority !== null && authority !== '') {
      params.set('authority', authority);
    }

    let response: Response;
    try {
      response = await this.fetchFn(`${this.baseUrl}/v1/applications/search?${params.toString()}`);
    } catch {
      // fetch() itself rejecting (offline, DNS failure, CORS, etc.) throws an
      // engine-specific, unfriendly message ("Failed to fetch", "NetworkError
      // when attempting to fetch resource", "Load failed") — never surface
      // that verbatim to an anonymous visitor of a public page.
      throw new Error('Could not reach the search service. Check your connection and try again.');
    }

    if (!response.ok) {
      throw new Error('Failed to search applications');
    }

    const dto = (await response.json()) as SearchResponseDto;
    return {
      results: dto.results.map(mapToDomain),
      refineQuery: dto.refineQuery,
    };
  }
}
