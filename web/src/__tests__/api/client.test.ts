import { describe, it, expect } from 'vitest';

interface FetchCall {
  url: string;
  init: RequestInit;
}

function createFakeFetch(status: number, body: unknown): { fetch: typeof globalThis.fetch; calls: FetchCall[] } {
  const calls: FetchCall[] = [];
  const fakeFetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init: init ?? {} });
    return new Response(JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    });
  };
  return { fetch: fakeFetch as typeof globalThis.fetch, calls };
}

import { createApiClient, ApiRequestError } from '../../api/client';

describe('createApiClient', () => {
  const baseUrl = 'https://api.example.com';
  const getToken = async () => 'test-token-123';

  it('sends GET request with auth header', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(200, { status: 'ok' });
    const api = createApiClient(baseUrl, getToken, fakeFetch);

    const result = await api.get<{ status: string }>('/health');

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe('https://api.example.com/health');
    expect(calls[0]!.init.headers).toEqual(
      expect.objectContaining({ Authorization: 'Bearer test-token-123' }),
    );
    expect(result).toEqual({ status: 'ok' });
  });

  it('sends POST request with JSON body', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(200, { id: '1' });
    const api = createApiClient(baseUrl, getToken, fakeFetch);

    await api.post<{ id: string }>('/items', { name: 'test' });

    expect(calls[0]!.init.method).toBe('POST');
    expect(calls[0]!.init.headers).toEqual(
      expect.objectContaining({ 'Content-Type': 'application/json' }),
    );
    expect(calls[0]!.init.body).toBe(JSON.stringify({ name: 'test' }));
  });

  it('sends DELETE request and handles 204 No Content', async () => {
    const fakeFetch = (async () =>
      new Response(null, { status: 204 })) as typeof globalThis.fetch;
    const api = createApiClient(baseUrl, getToken, fakeFetch);

    const result = await api.delete('/items/1');

    expect(result).toBeUndefined();
  });

  it('throws ApiRequestError on 4xx response', async () => {
    const { fetch: fakeFetch } = createFakeFetch(404, { error: 'Not found' });
    const api = createApiClient(baseUrl, getToken, fakeFetch);

    await expect(api.get('/missing')).rejects.toThrow(ApiRequestError);
    await expect(api.get('/missing')).rejects.toMatchObject({
      status: 404,
      message: 'Not found',
    });
  });

  it('appends query parameters to GET requests', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(200, {});
    const api = createApiClient(baseUrl, getToken, fakeFetch);

    await api.get<unknown>('/search', { q: 'test', page: '1' });

    expect(calls[0]!.url).toBe('https://api.example.com/search?q=test&page=1');
  });

  it('sends PATCH request with JSON body', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(200, { userId: '1' });
    const api = createApiClient(baseUrl, getToken, fakeFetch);

    await api.patch<{ userId: string }>('/me', { postcode: 'SW1A 1AA' });

    expect(calls[0]!.init.method).toBe('PATCH');
    expect(calls[0]!.init.body).toBe(JSON.stringify({ postcode: 'SW1A 1AA' }));
  });

  it('sends PUT request with JSON body', async () => {
    const calls: FetchCall[] = [];
    const fakeFetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ url: String(input), init: init ?? {} });
      return new Response(null, { status: 204 });
    };
    const api = createApiClient(baseUrl, getToken, fakeFetch as typeof globalThis.fetch);

    await api.put('/items/1', { name: 'updated' });

    expect(calls[0]!.init.method).toBe('PUT');
  });
});
