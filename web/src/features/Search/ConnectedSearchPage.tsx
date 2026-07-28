import { useMemo } from 'react';
import { ApiSearchPort } from './ApiSearchPort';
import { PostcodesIoLookupPort } from './PostcodesIoLookupPort';
import { SearchPage } from './SearchPage';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string || 'http://localhost:5000';

/**
 * Composition root for the public `/search` route (#821 Phase 4). Wires the
 * anonymous `ApiSearchPort` directly — never the token-bearing `ApiClient` —
 * so this page keeps working for a visitor who has never signed in. Also
 * wires `PostcodesIoLookupPort` (GH#863, tc-rrv7i.2), which talks to
 * `api.postcodes.io` directly from the browser rather than through
 * `API_BASE_URL` — postcode lookups must never be proxied through our own
 * server (see the port's own doc comment for why).
 */
export function ConnectedSearchPage() {
  const port = useMemo(() => new ApiSearchPort(API_BASE_URL), []);
  const postcodePort = useMemo(() => new PostcodesIoLookupPort(), []);

  return <SearchPage port={port} postcodePort={postcodePort} />;
}
