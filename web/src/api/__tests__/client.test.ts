import { describe, it, expect } from 'vitest';
import { createApiClient } from '../client';

interface RecordedRequest {
  readonly url: string;
  readonly init: RequestInit | undefined;
}

/**
 * Hand-written fetch spy — records the request and returns a caller-supplied
 * Response. No `vi.fn()`, in keeping with the no-mocking-frameworks policy.
 */
class SpyFetch {
  calls: RecordedRequest[] = [];
  response: Response = new Response('null', { status: 200 });

  readonly fn = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    this.calls.push({ url: String(input), init });
    return Promise.resolve(this.response);
  };
}

function clientWith(spy: SpyFetch) {
  return createApiClient('https://api.test', () => Promise.resolve('tok-123'), spy.fn);
}

describe('createApiClient — getWithHeaders', () => {
  it('returns the parsed body alongside the raw response headers', async () => {
    const spy = new SpyFetch();
    spy.response = new Response(JSON.stringify([{ uid: 'A' }]), {
      status: 200,
      headers: { 'X-Next-Cursor': 'cursor-page-2' },
    });
    const client = clientWith(spy);

    const { body, headers } = await client.getWithHeaders<readonly { uid: string }[]>(
      '/v1/me/watch-zones/z1/applications',
    );

    expect(body).toEqual([{ uid: 'A' }]);
    expect(headers.get('X-Next-Cursor')).toBe('cursor-page-2');
  });

  it('exposes a null X-Next-Cursor header when the response omits it (last page)', async () => {
    const spy = new SpyFetch();
    spy.response = new Response(JSON.stringify([]), { status: 200 });
    const client = clientWith(spy);

    const { headers } = await client.getWithHeaders('/v1/me/watch-zones/z1/applications');

    expect(headers.get('X-Next-Cursor')).toBeNull();
  });

  it('sends the bearer token and serialises params into the query string', async () => {
    const spy = new SpyFetch();
    spy.response = new Response(JSON.stringify([]), { status: 200 });
    const client = clientWith(spy);

    await client.getWithHeaders('/v1/me/watch-zones/z1/applications', {
      sort: 'newest',
      cursor: 'abc',
    });

    const call = spy.calls[0]!;
    expect(call.url).toBe(
      'https://api.test/v1/me/watch-zones/z1/applications?sort=newest&cursor=abc',
    );
    const authHeader = (call.init?.headers as Record<string, string>)['Authorization'];
    expect(authHeader).toBe('Bearer tok-123');
  });

  it('throws ApiRequestError carrying the status on a non-2xx response', async () => {
    const spy = new SpyFetch();
    spy.response = new Response(JSON.stringify({ error: 'bad cursor' }), { status: 400 });
    const client = clientWith(spy);

    await expect(
      client.getWithHeaders('/v1/me/watch-zones/z1/applications', { cursor: 'stale' }),
    ).rejects.toMatchObject({ status: 400, message: 'bad cursor' });
  });

  it('leaves get() returning the bare parsed body (unchanged contract)', async () => {
    const spy = new SpyFetch();
    spy.response = new Response(JSON.stringify({ ok: true }), {
      status: 200,
      headers: { 'X-Next-Cursor': 'ignored' },
    });
    const client = clientWith(spy);

    const body = await client.get<{ ok: boolean }>('/v1/anything');

    expect(body).toEqual({ ok: true });
  });
});
