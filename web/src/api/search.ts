import type { ApiClient } from './client';
import type { SearchResult } from '../domain/types';

export function searchApi(client: ApiClient) {
  return {
    search: (q: string, authorityId: number, page: number = 1) =>
      client.get<SearchResult>('/v1/search', {
        q,
        authorityId: String(authorityId),
        page: String(page),
      }),
  };
}
