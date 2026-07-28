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
 * Client-side adapter for postcodes.io (GH#863, tc-rrv7i.2). Calls
 * `https://api.postcodes.io` directly from the browser using its own
 * `fetch` — this is a hard constraint, not a style choice: postcodes.io is a
 * free public service, and proxying every user's lookup through our own
 * server would put all of them behind one server IP, risking
 * rate-limiting/blacklisting for every user at once. Never wire this through
 * the app's own `ApiClient`/base URL.
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
      // fetch() itself rejecting (offline, DNS failure, CORS, etc.) throws an
      // engine-specific, unfriendly message — never surface that verbatim.
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
