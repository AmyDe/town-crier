import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { searchApi } from '../../api/search';
import { authoritiesApi } from '../../api/authorities';
import type { SearchRepository } from '../../domain/ports/search-repository';
import type { AuthoritySearchPort } from '../../domain/ports/authority-search-port';
import { SearchPage } from './SearchPage';

export function ConnectedSearchPage() {
  const client = useApiClient();

  const searchRepository: SearchRepository = useMemo(
    () => ({
      search: (query, authorityId, page) =>
        searchApi(client).search(query, authorityId, page),
    }),
    [client],
  );

  const authoritySearchPort: AuthoritySearchPort = useMemo(
    () => ({
      search: (query) => authoritiesApi(client).list(query),
    }),
    [client],
  );

  return (
    <SearchPage
      searchRepository={searchRepository}
      authoritySearchPort={authoritySearchPort}
    />
  );
}
