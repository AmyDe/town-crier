import { describe, it, expect } from 'vitest';
import type { ApiClient } from '../client';
import { applicationsApi } from '../applications';
import type { PlanningApplicationSummary } from '../../domain/types';
import { asApplicationUid } from '../../domain/types';

interface RecordedGet {
  readonly path: string;
  readonly params: Record<string, string> | undefined;
}

/**
 * Hand-written ApiClient spy. Only `get`/`getWithHeaders` are exercised by the
 * applications API; the rest throw so an accidental call is loud.
 */
class SpyApiClient implements ApiClient {
  getCalls: RecordedGet[] = [];
  getWithHeadersCalls: RecordedGet[] = [];

  getResult: unknown = [];
  getWithHeadersBody: unknown = [];
  getWithHeadersCursor: string | null = null;

  async get<T>(path: string, params?: Record<string, string>): Promise<T> {
    this.getCalls.push({ path, params });
    return this.getResult as T;
  }

  async getWithHeaders<T>(
    path: string,
    params?: Record<string, string>,
  ): Promise<{ body: T; headers: Headers }> {
    this.getWithHeadersCalls.push({ path, params });
    const headers = new Headers();
    if (this.getWithHeadersCursor !== null) {
      headers.set('X-Next-Cursor', this.getWithHeadersCursor);
    }
    return { body: this.getWithHeadersBody as T, headers };
  }

  post<T>(): Promise<T> {
    throw new Error('not implemented');
  }
  put(): Promise<void> {
    throw new Error('not implemented');
  }
  patch<T>(): Promise<T> {
    throw new Error('not implemented');
  }
  delete(): Promise<void> {
    throw new Error('not implemented');
  }
}

function summary(uid: string): PlanningApplicationSummary {
  return {
    uid: asApplicationUid(uid),
    name: uid,
    address: '1 Test Street',
    postcode: null,
    description: 'Test',
    appType: 'Full Planning',
    appState: 'Undecided',
    areaName: 'Cambridge City Council',
    startDate: '2026-01-01',
    url: null,
    latitude: null,
    longitude: null,
    latestUnreadEvent: null,
  };
}

describe('applicationsApi.getByZonePaged', () => {
  it('sends sort/cursor/limit params and returns rows plus the X-Next-Cursor header', async () => {
    const client = new SpyApiClient();
    client.getWithHeadersBody = [summary('A'), summary('B')];
    client.getWithHeadersCursor = 'cursor-2';

    const page = await applicationsApi(client).getByZonePaged('z1', {
      sort: 'newest',
      status: null,
      unread: false,
      cursor: 'cursor-1',
      limit: 150,
    });

    expect(page.rows).toHaveLength(2);
    expect(page.nextCursor).toBe('cursor-2');
    const call = client.getWithHeadersCalls[0]!;
    expect(call.path).toBe('/v1/me/watch-zones/z1/applications');
    expect(call.params).toEqual({ sort: 'newest', cursor: 'cursor-1', limit: '150' });
  });

  it('returns a null nextCursor when the response omits the header (last page)', async () => {
    const client = new SpyApiClient();
    client.getWithHeadersBody = [summary('A')];
    client.getWithHeadersCursor = null;

    const page = await applicationsApi(client).getByZonePaged('z1', {
      sort: 'distance',
      status: null,
      unread: false,
      cursor: null,
    });

    expect(page.nextCursor).toBeNull();
    expect(client.getWithHeadersCalls[0]!.params).toEqual({ sort: 'distance' });
  });

  it('sends status= when a status filter is set', async () => {
    const client = new SpyApiClient();
    await applicationsApi(client).getByZonePaged('z1', {
      sort: 'status',
      status: 'Permitted',
      unread: false,
      cursor: null,
    });

    expect(client.getWithHeadersCalls[0]!.params).toEqual({
      sort: 'status',
      status: 'Permitted',
    });
  });

  it('sends unread=true and never status when unread-only is set (mutually exclusive)', async () => {
    const client = new SpyApiClient();
    await applicationsApi(client).getByZonePaged('z1', {
      sort: 'recent-activity',
      // Even if a stale status slips through, unread wins and status is dropped.
      status: 'Permitted',
      unread: true,
      cursor: null,
    });

    const params = client.getWithHeadersCalls[0]!.params!;
    expect(params['unread']).toBe('true');
    expect(params['status']).toBeUndefined();
  });
});

describe('applicationsApi.getByZone (param-less, unchanged)', () => {
  it('issues a header-blind GET with no params for the backward-compatible call', async () => {
    const client = new SpyApiClient();
    client.getResult = [summary('A')];

    const rows = await applicationsApi(client).getByZone('z1');

    expect(rows).toHaveLength(1);
    expect(client.getWithHeadersCalls).toHaveLength(0);
    expect(client.getCalls[0]).toEqual({
      path: '/v1/me/watch-zones/z1/applications',
      params: undefined,
    });
  });
});
