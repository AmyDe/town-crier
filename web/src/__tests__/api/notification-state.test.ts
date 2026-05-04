import { describe, it, expect } from 'vitest';
import { notificationStateApi } from '../../api/notification-state';
import { createApiClient } from '../../api/client';

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

describe('notificationStateApi.getState', () => {
  it('GETs /v1/me/notification-state and returns the wire snapshot', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(200, {
      lastReadAt: '2026-05-04T12:00:00Z',
      version: 3,
      totalUnreadCount: 7,
    });
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    const result = await notificationStateApi(client).getState();

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe('https://api.example.com/v1/me/notification-state');
    expect(calls[0]!.init.method).toBe('GET');
    expect(result).toEqual({
      lastReadAt: '2026-05-04T12:00:00Z',
      version: 3,
      totalUnreadCount: 7,
    });
  });
});

describe('notificationStateApi.markAllRead', () => {
  it('POSTs /v1/me/notification-state/mark-all-read with no body', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(204, null);
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    await notificationStateApi(client).markAllRead();

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(
      'https://api.example.com/v1/me/notification-state/mark-all-read',
    );
    expect(calls[0]!.init.method).toBe('POST');
    expect(calls[0]!.init.body).toBeUndefined();
  });
});

describe('notificationStateApi.advance', () => {
  it('POSTs /v1/me/notification-state/advance with the asOf instant in the body', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(204, null);
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    await notificationStateApi(client).advance('2026-05-04T12:00:00Z');

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(
      'https://api.example.com/v1/me/notification-state/advance',
    );
    expect(calls[0]!.init.method).toBe('POST');
    expect(calls[0]!.init.body).toBe(
      JSON.stringify({ asOf: '2026-05-04T12:00:00Z' }),
    );
  });
});
