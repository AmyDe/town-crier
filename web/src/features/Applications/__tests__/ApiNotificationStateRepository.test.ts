import { describe, it, expect } from 'vitest';
import { ApiNotificationStateRepository } from '../ApiNotificationStateRepository';
import { createApiClient } from '../../../api/client';

interface FetchCall {
  url: string;
  init: RequestInit;
}

function createFakeFetch(
  status: number,
  body: unknown,
): { fetch: typeof globalThis.fetch; calls: FetchCall[] } {
  const calls: FetchCall[] = [];
  const fakeFetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init: init ?? {} });
    return new Response(body === null ? null : JSON.stringify(body), {
      status,
      headers: body === null ? undefined : { 'Content-Type': 'application/json' },
    });
  };
  return { fetch: fakeFetch as typeof globalThis.fetch, calls };
}

const baseUrl = 'https://api.example.com';
const getToken = async () => 'token';

describe('ApiNotificationStateRepository', () => {
  it('getState returns the wire snapshot from /v1/me/notification-state', async () => {
    const { fetch: fakeFetch } = createFakeFetch(200, {
      lastReadAt: '2026-05-04T12:00:00Z',
      version: 4,
      totalUnreadCount: 12,
    });
    const client = createApiClient(baseUrl, getToken, fakeFetch);
    const repo = new ApiNotificationStateRepository(client);

    const result = await repo.getState();

    expect(result).toEqual({
      lastReadAt: '2026-05-04T12:00:00Z',
      version: 4,
      totalUnreadCount: 12,
    });
  });

  it('markAllRead POSTs to /v1/me/notification-state/mark-all-read', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(204, null);
    const client = createApiClient(baseUrl, getToken, fakeFetch);
    const repo = new ApiNotificationStateRepository(client);

    await repo.markAllRead();

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(
      'https://api.example.com/v1/me/notification-state/mark-all-read',
    );
    expect(calls[0]!.init.method).toBe('POST');
  });

  it('advance POSTs the asOf instant', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(204, null);
    const client = createApiClient(baseUrl, getToken, fakeFetch);
    const repo = new ApiNotificationStateRepository(client);

    await repo.advance('2026-05-04T12:00:00Z');

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(
      'https://api.example.com/v1/me/notification-state/advance',
    );
    expect(calls[0]!.init.body).toBe(
      JSON.stringify({ asOf: '2026-05-04T12:00:00Z' }),
    );
  });
});
