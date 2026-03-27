import type { AuthorityId, SearchResult } from '../../../../domain/types';
import type { SearchRepository } from '../../../../domain/ports/search-repository';

export class SpySearchRepository implements SearchRepository {
  searchCalls: Array<{ query: string; authorityId: AuthorityId; page: number }> = [];
  searchResult: SearchResult = { applications: [], total: 0, page: 1 };
  searchError: Error | null = null;

  async search(query: string, authorityId: AuthorityId, page: number): Promise<SearchResult> {
    this.searchCalls.push({ query, authorityId, page });
    if (this.searchError) {
      throw this.searchError;
    }
    return this.searchResult;
  }
}
