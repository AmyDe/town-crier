import type { SearchOutcome, SearchPort } from '../../../../domain/ports/search-port';

interface SearchCall {
  readonly query: string;
  readonly authority: string | null;
}

export class SpySearchPort implements SearchPort {
  searchCalls: SearchCall[] = [];
  searchResult: SearchOutcome = { results: [], refineQuery: false };
  searchError: Error | null = null;

  async search(query: string, authority: string | null): Promise<SearchOutcome> {
    this.searchCalls.push({ query, authority });
    if (this.searchError) {
      throw this.searchError;
    }
    return this.searchResult;
  }
}
