import { describe, it, expect, beforeEach } from 'vitest';
import { PostcodesIoLookupPort } from '../PostcodesIoLookupPort';

class StubFetch {
  lastUrl: string | null = null;
  responseBody: unknown = { status: 200, result: { latitude: 51.5, longitude: -0.1 } };
  responseStatus = 200;
  shouldReject = false;
  rejectError: Error = new Error('Network failure');

  readonly fetch: typeof globalThis.fetch = async (input: RequestInfo | URL) => {
    this.lastUrl = String(input);
    if (this.shouldReject) {
      throw this.rejectError;
    }
    return {
      ok: this.responseStatus >= 200 && this.responseStatus < 300,
      status: this.responseStatus,
      json: async () => this.responseBody,
    } as Response;
  };
}

describe('PostcodesIoLookupPort', () => {
  let stub: StubFetch;
  let port: PostcodesIoLookupPort;

  beforeEach(() => {
    stub = new StubFetch();
    port = new PostcodesIoLookupPort(stub.fetch);
  });

  it('calls api.postcodes.io directly — never our own API baseUrl', async () => {
    await port.lookup('SW1A 1AA');

    expect(stub.lastUrl).toBe('https://api.postcodes.io/postcodes/SW1A%201AA');
  });

  it('resolves lat/lon from a successful lookup', async () => {
    stub.responseBody = {
      status: 200,
      result: { latitude: 51.501, longitude: -0.1415 },
    };

    const location = await port.lookup('SW1A 1AA');

    expect(location).toEqual({ lat: 51.501, lon: -0.1415 });
  });

  it('trims whitespace from the postcode before encoding it', async () => {
    await port.lookup('  SW1A 1AA  ');

    expect(stub.lastUrl).toBe('https://api.postcodes.io/postcodes/SW1A%201AA');
  });

  it('throws a distinct "not found" message on a 404', async () => {
    stub.responseStatus = 404;
    stub.responseBody = { status: 404, error: 'Postcode not found' };

    await expect(port.lookup('ZZ9 9ZZ')).rejects.toThrow('Postcode not found');
  });

  it('throws a generic message on a non-404 non-ok response', async () => {
    stub.responseStatus = 500;

    await expect(port.lookup('SW1A 1AA')).rejects.toThrow(
      'Could not look up that postcode. Check your connection and try again.',
    );
  });

  it('throws a friendly message when fetch itself fails (offline, DNS, CORS, etc.)', async () => {
    stub.shouldReject = true;
    stub.rejectError = new TypeError('Failed to fetch');

    await expect(port.lookup('SW1A 1AA')).rejects.toThrow(
      'Could not look up that postcode. Check your connection and try again.',
    );
  });
});
