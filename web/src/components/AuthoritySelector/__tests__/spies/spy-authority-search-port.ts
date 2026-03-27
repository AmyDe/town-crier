import type { AuthoritiesResult } from '../../../../domain/types';
import type { AuthoritySearchPort } from '../../../../domain/ports/authority-search-port';

export class SpyAuthoritySearchPort implements AuthoritySearchPort {
  searchCalls: string[] = [];
  searchResult: AuthoritiesResult = { authorities: [], total: 0 };
  searchError: Error | null = null;

  async search(query: string): Promise<AuthoritiesResult> {
    this.searchCalls.push(query);
    if (this.searchError) {
      throw this.searchError;
    }
    return this.searchResult;
  }
}
