import { describe, it, expect } from 'vitest';
import { applicationsApi } from '../../api/applications';
import { createApiClient } from '../../api/client';

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

describe('applicationsApi.getClusters', () => {
  it('GETs the zone clusters endpoint with bbox and zoom query params', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(200, []);
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    await applicationsApi(client).getClusters('zone-1', {
      bbox: '-0.5,51.0,0.5,52.0',
      zoom: 13,
    });

    expect(calls).toHaveLength(1);
    const url = new URL(calls[0]!.url);
    expect(url.pathname).toBe('/v1/me/watch-zones/zone-1/applications/clusters');
    expect(url.searchParams.get('bbox')).toBe('-0.5,51.0,0.5,52.0');
    expect(url.searchParams.get('zoom')).toBe('13');
    expect(url.searchParams.has('status')).toBe(false);
    expect(calls[0]!.init.method).toBe('GET');
  });

  it('includes the status param when a status filter is supplied', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(200, []);
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    await applicationsApi(client).getClusters('zone-1', {
      bbox: '-0.5,51.0,0.5,52.0',
      zoom: 13,
      status: 'Permitted',
    });

    const url = new URL(calls[0]!.url);
    expect(url.searchParams.get('status')).toBe('Permitted');
  });

  it('omits the status param when status is null', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(200, []);
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    await applicationsApi(client).getClusters('zone-1', {
      bbox: '-0.5,51.0,0.5,52.0',
      zoom: 13,
      status: null,
    });

    const url = new URL(calls[0]!.url);
    expect(url.searchParams.has('status')).toBe(false);
  });

  it('maps a count>1 cell to a bubble with member null', async () => {
    const { fetch: fakeFetch } = createFakeFetch(200, [
      {
        latitude: 52.2,
        longitude: 0.12,
        count: 5,
        statusCounts: { Undecided: 3, Permitted: 2 },
        applicationId: null,
      },
    ]);
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    const clusters = await applicationsApi(client).getClusters('zone-1', {
      bbox: '0,0,1,1',
      zoom: 10,
    });

    expect(clusters).toHaveLength(1);
    expect(clusters[0]).toEqual({
      latitude: 52.2,
      longitude: 0.12,
      count: 5,
      statusCounts: { Undecided: 3, Permitted: 2 },
      member: null,
    });
  });

  it('maps a count==1 cell to a pin carrying its {authority,name} member', async () => {
    const { fetch: fakeFetch } = createFakeFetch(200, [
      {
        latitude: 52.2,
        longitude: 0.12,
        count: 1,
        statusCounts: { Permitted: 1 },
        applicationId: { authority: '42', name: '22/1234/FUL' },
      },
    ]);
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    const clusters = await applicationsApi(client).getClusters('zone-1', {
      bbox: '0,0,1,1',
      zoom: 18,
    });

    expect(clusters[0]!.member).toEqual({ authority: '42', name: '22/1234/FUL' });
    expect(clusters[0]!.count).toBe(1);
  });
});

describe('applicationsApi.getByAuthorityAndName', () => {
  it('point-reads /v1/applications/{authority}/{name} preserving slashes in name', async () => {
    const { fetch: fakeFetch, calls } = createFakeFetch(200, {
      uid: 'CAM/2026/0042',
      name: '22/1234/FUL',
      areaId: 42,
    });
    const client = createApiClient(baseUrl, getToken, fakeFetch);

    await applicationsApi(client).getByAuthorityAndName('42', '22/1234/FUL');

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe('https://api.example.com/v1/applications/42/22/1234/FUL');
    expect(calls[0]!.init.method).toBe('GET');
  });
});
