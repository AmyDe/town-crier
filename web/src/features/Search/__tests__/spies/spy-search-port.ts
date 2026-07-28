import type { SearchLocation, SearchOutcome, SearchPort } from '../../../../domain/ports/search-port';

interface SearchCall {
  readonly query: string;
  readonly authority: string | null;
  readonly location: SearchLocation;
}

export class SpySearchPort implements SearchPort {
  searchCalls: SearchCall[] = [];
  searchResult: SearchOutcome = { results: [], refineQuery: false };
  searchError: Error | null = null;

  async search(
    query: string,
    authority: string | null,
    location: SearchLocation,
  ): Promise<SearchOutcome> {
    this.searchCalls.push({ query, authority, location });
    if (this.searchError) {
      throw this.searchError;
    }
    return this.searchResult;
  }
}
