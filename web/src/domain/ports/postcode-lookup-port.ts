import type { SearchLocation } from './search-port';

/**
 * Client-side postcode -> lat/lon resolution for the `/search` location
 * picker (GH#863, tc-rrv7i.2). Deliberately never routed through our own
 * `ApiClient`/base URL — the concrete adapter must call postcodes.io
 * directly from the browser (see `PostcodesIoLookupPort`), so a spike in
 * lookups is spread across every visitor's own IP rather than funnelled
 * through our single server IP.
 */
export interface PostcodeLookupPort {
  /**
   * @param postcode raw user input; the adapter trims/encodes it.
   * @throws when the postcode does not resolve (distinct message from a
   *   network failure) or the lookup itself could not be made.
   */
  lookup(postcode: string): Promise<SearchLocation>;
}
