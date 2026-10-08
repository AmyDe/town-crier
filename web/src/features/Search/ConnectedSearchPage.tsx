import { useMemo } from 'react';
import { ApiSearchPort } from './ApiSearchPort';
import { PostcodesIoLookupPort } from './PostcodesIoLookupPort';
import { SearchPage } from './SearchPage';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string || 'http://localhost:5000';

/**
 * Wires the anonymous `ApiSearchPort`, never the token-bearing `ApiClient`, so
 * the page works for a visitor who has never signed in.
 */
export function ConnectedSearchPage() {
  const port = useMemo(() => new ApiSearchPort(API_BASE_URL), []);
  const postcodePort = useMemo(() => new PostcodesIoLookupPort(), []);

  return <SearchPage port={port} postcodePort={postcodePort} />;
}
