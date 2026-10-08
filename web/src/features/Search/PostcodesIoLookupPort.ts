import type { PostcodeLookupPort } from '../../domain/ports/postcode-lookup-port';
import type { SearchLocation } from '../../domain/ports/search-port';

const POSTCODES_IO_BASE_URL = 'https://api.postcodes.io';

interface PostcodesIoResult {
  readonly latitude: number;
  readonly longitude: number;
}

interface PostcodesIoResponse {
  readonly status: number;
  readonly result: PostcodesIoResult | null;
}

/**
 * Calls postcodes.io directly from the browser, never through our own API:
 * proxying every lookup through one server IP risks rate-limiting all users at once.
 */
export class PostcodesIoLookupPort implements PostcodeLookupPort {
  private readonly fetchFn: typeof globalThis.fetch;

  constructor(fetchFn: typeof globalThis.fetch = globalThis.fetch.bind(globalThis)) {
    this.fetchFn = fetchFn;
  }

  async lookup(postcode: string): Promise<SearchLocation> {
    const normalised = postcode.trim();

    let response: Response;
    try {
      response = await this.fetchFn(
        `${POSTCODES_IO_BASE_URL}/postcodes/${encodeURIComponent(normalised)}`,
      );
    } catch {
      // A fetch() rejection carries engine-specific text, so surface a friendly message.
      throw new Error('Could not look up that postcode. Check your connection and try again.');
    }

    if (response.status === 404) {
      throw new Error('Postcode not found');
    }
    if (!response.ok) {
      throw new Error('Could not look up that postcode. Check your connection and try again.');
    }

    const dto = (await response.json()) as PostcodesIoResponse;
    if (!dto.result) {
      throw new Error('Postcode not found');
    }
    return { lat: dto.result.latitude, lon: dto.result.longitude };
  }
}
