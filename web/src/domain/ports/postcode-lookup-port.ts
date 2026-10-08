import type { SearchLocation } from './search-port';

/**
 * Client-side postcode -> lat/lon resolution for the `/search` location picker.
 * Implementations must call postcodes.io from the browser, never through our own API.
 */
export interface PostcodeLookupPort {
  /**
   * @param postcode raw user input; the adapter trims/encodes it.
   * @throws when the postcode does not resolve (distinct message from a
   *   network failure) or the lookup itself could not be made.
   */
  lookup(postcode: string): Promise<SearchLocation>;
}
