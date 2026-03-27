import type { AuthorityId, SearchResult } from '../types';

export interface SearchRepository {
  search(query: string, authorityId: AuthorityId, page: number): Promise<SearchResult>;
}
