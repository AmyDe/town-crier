import type { SearchResult } from '../../../../domain/types';

export function aSearchResult(overrides?: Partial<SearchResult>): SearchResult {
  return {
    reference: '22/1234/FUL',
    authoritySlug: 'cambridge',
    authorityName: 'Cambridge City Council',
    address: '12 Mill Road, Cambridge, CB1 2AD',
    appState: 'Permitted',
    startDate: '2026-01-15',
    decidedDate: '2026-03-01',
    ...overrides,
  };
}

export function anotherSearchResult(overrides?: Partial<SearchResult>): SearchResult {
  return aSearchResult({
    reference: '24/0001/FUL',
    authoritySlug: 'adur',
    authorityName: 'Adur District Council',
    address: '1 High Street, Shoreham-by-Sea, BN43 5WU',
    appState: null,
    startDate: null,
    decidedDate: null,
    ...overrides,
  });
}
