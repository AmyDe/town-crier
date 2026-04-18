import { describe, it, expect } from 'vitest';
import { createRedeemOfferCodeClient } from '../api/redeemOfferCode';
import { RedeemError, type RedeemErrorCode } from '../api/types';

interface RecordedRequest {
  readonly url: string;
  readonly method: string;
  readonly headers: Record<string, string>;
  readonly body: string | null;
}

function createFakeFetch(
  response: Response,
): { fetch: typeof globalThis.fetch; requests: RecordedRequest[] } {
  const requests: RecordedRequest[] = [];
  const fetchFn: typeof globalThis.fetch = async (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const method = init?.method ?? 'GET';
    const headerEntries: Record<string, string> = {};
    const rawHeaders = init?.headers;
    if (rawHeaders instanceof Headers) {
      rawHeaders.forEach((v, k) => { headerEntries[k.toLowerCase()] = v; });
    } else if (Array.isArray(rawHeaders)) {
      for (const [k, v] of rawHeaders) headerEntries[k.toLowerCase()] = v;
    } else if (rawHeaders) {
      for (const [k, v] of Object.entries(rawHeaders)) headerEntries[k.toLowerCase()] = String(v);
    }
    const body = typeof init?.body === 'string' ? init.body : null;
    requests.push({ url, method, headers: headerEntries, body });
    return response;
  };
  return { fetch: fetchFn, requests };
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('createRedeemOfferCodeClient', () => {
  const apiBaseUrl = 'https://api.example.test';
  const getAccessToken = async (): Promise<string> => 'token-abc';

  it('POSTs to the redeem endpoint with the access token and code body', async () => {
    const { fetch, requests } = createFakeFetch(
      jsonResponse(200, { tier: 'Pro', expiresAt: '2026-05-18T00:00:00Z' }),
    );
    const client = createRedeemOfferCodeClient(getAccessToken, apiBaseUrl, fetch);

    await client('A7KM-ZQR3-FNXP');

    expect(requests).toHaveLength(1);
    const [request] = requests;
    expect(request).toBeDefined();
    expect(request!.url).toBe('https://api.example.test/v1/offer-codes/redeem');
    expect(request!.method).toBe('POST');
    expect(request!.headers['authorization']).toBe('Bearer token-abc');
    expect(request!.headers['content-type']).toBe('application/json');
    expect(request!.body).toBe(JSON.stringify({ code: 'A7KM-ZQR3-FNXP' }));
  });

  it('returns the parsed RedeemResult on a 200 response', async () => {
    const { fetch } = createFakeFetch(
      jsonResponse(200, { tier: 'Personal', expiresAt: '2026-06-01T00:00:00Z' }),
    );
    const client = createRedeemOfferCodeClient(getAccessToken, apiBaseUrl, fetch);

    const result = await client('VALID-CODE-0001');

    expect(result).toEqual({ tier: 'Personal', expiresAt: '2026-06-01T00:00:00Z' });
  });

  it.each<[number, string, RedeemErrorCode]>([
    [400, 'invalid_code_format', 'invalid_code_format'],
    [404, 'invalid_code', 'invalid_code'],
    [409, 'code_already_redeemed', 'code_already_redeemed'],
    [409, 'already_subscribed', 'already_subscribed'],
  ])('maps server error code %s/%s to RedeemError(%s)', async (status, serverCode, expectedCode) => {
    const { fetch } = createFakeFetch(jsonResponse(status, { error: serverCode }));
    const client = createRedeemOfferCodeClient(getAccessToken, apiBaseUrl, fetch);

    const promise = client('A7KM-ZQR3-FNXP');

    await expect(promise).rejects.toBeInstanceOf(RedeemError);
    await expect(promise).rejects.toMatchObject({ code: expectedCode });
  });

  it('falls back to network error code when the server error is unrecognised', async () => {
    const { fetch } = createFakeFetch(jsonResponse(500, { error: 'internal_explosion' }));
    const client = createRedeemOfferCodeClient(getAccessToken, apiBaseUrl, fetch);

    const promise = client('A7KM-ZQR3-FNXP');

    await expect(promise).rejects.toBeInstanceOf(RedeemError);
    await expect(promise).rejects.toMatchObject({ code: 'network' });
  });

  it('falls back to network error code when the response body is not JSON', async () => {
    const response = new Response('oops', { status: 502 });
    const { fetch } = createFakeFetch(response);
    const client = createRedeemOfferCodeClient(getAccessToken, apiBaseUrl, fetch);

    const promise = client('A7KM-ZQR3-FNXP');

    await expect(promise).rejects.toBeInstanceOf(RedeemError);
    await expect(promise).rejects.toMatchObject({ code: 'network' });
  });
});
