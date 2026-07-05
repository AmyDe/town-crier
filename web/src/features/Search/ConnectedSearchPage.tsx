import { useMemo } from 'react';
import { ApiSearchPort } from './ApiSearchPort';
import { SearchPage } from './SearchPage';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string || 'http://localhost:5000';

/**
 * Composition root for the public `/search` route (#821 Phase 4). Wires the
 * anonymous `ApiSearchPort` directly — never the token-bearing `ApiClient` —
 * so this page keeps working for a visitor who has never signed in.
 */
export function ConnectedSearchPage() {
  const port = useMemo(() => new ApiSearchPort(API_BASE_URL), []);

  return <SearchPage port={port} />;
}
