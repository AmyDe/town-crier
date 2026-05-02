import { describe, it, expect } from 'vitest';
import { savedApplicationsApi } from '../../api/savedApplications';
import { createApiClient } from '../../api/client';
import type { PlanningApplication } from '../../domain/types';
import { asApplicationUid, asAuthorityId } from '../../domain/types';

interface FetchCall {
  url: string;
  init: RequestInit;
}

function createFakeFetch(status: number, body: unknown): {
  fetch: typeof globalThis.fetch;
  calls: FetchCall[];
} {
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

function anApplication(overrides?: Partial<PlanningApplication>): PlanningApplication {
  return {
    uid: asApplicationUid('CAM/2026/0042'),
    name: '2026/0042/FUL',
    areaName: 'Cambridge City Council',
    areaId: asAuthorityId(42),
    address: '12 Mill Road, Cambridge, CB1 2AD',
    postcode: 'CB1 2AD',
    description: 'Erection of two-storey rear extension',
    appType: 'Full Planning',
    appState: 'Undecided',
    appSize: null,
    startDate: '2026-01-15',
    decidedDate: null,
    consultedDate: null,
    longitude: 0.1218,
    latitude: 52.2053,
    url: 'https://council.example.com/planning/CAM-2026-0042',
    link: 'https://planit.org.uk/planapplic/CAM-2026-0042',
    lastDifferent: '2026-01-20',
    ...overrides,
  };
}

describe('savedApplicationsApi.save', () => {
  it('PUTs the full PlanningApplication to /v1/me/saved-applications/{uid}', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(204, null);
    const client = createApiClient(baseUrl, getToken, fakeFetch);
    const application = anApplication();

    await savedApplicationsApi(client).save(application);

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(
      'https://api.example.com/v1/me/saved-applications/CAM/2026/0042',
    );
    expect(calls[0]!.init.method).toBe('PUT');
    expect(calls[0]!.init.headers).toEqual(
      expect.objectContaining({ 'Content-Type': 'application/json' }),
    );
    expect(calls[0]!.init.body).toBe(JSON.stringify(application));
  });
});

describe('savedApplicationsApi.list', () => {
  it('GETs /v1/me/saved-applications', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(200, []);
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    await savedApplicationsApi(client).list();

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe('https://api.example.com/v1/me/saved-applications');
    expect(calls[0]!.init.method).toBe('GET');
  });
});

describe('savedApplicationsApi.remove', () => {
  it('DELETEs /v1/me/saved-applications/{uid}', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(204, null);
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    await savedApplicationsApi(client).remove(asApplicationUid('CAM/2026/0042'));

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(
      'https://api.example.com/v1/me/saved-applications/CAM/2026/0042',
    );
    expect(calls[0]!.init.method).toBe('DELETE');
  });
});
