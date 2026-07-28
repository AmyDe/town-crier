import type { PostcodeLookupPort } from '../../../../domain/ports/postcode-lookup-port';
import type { SearchLocation } from '../../../../domain/ports/search-port';

export class SpyPostcodeLookupPort implements PostcodeLookupPort {
  lookupCalls: string[] = [];
  lookupResult: SearchLocation = { lat: 51.5, lon: -0.1 };
  lookupError: Error | null = null;

  async lookup(postcode: string): Promise<SearchLocation> {
    this.lookupCalls.push(postcode);
    if (this.lookupError) {
      throw this.lookupError;
    }
    return this.lookupResult;
  }
}
