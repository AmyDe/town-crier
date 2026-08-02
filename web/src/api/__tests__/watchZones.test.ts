import { describe, it, expect } from 'vitest';
import { createApiClient } from '../client';
import { watchZonesApi } from '../watchZones';
import type { WatchZoneBoundary, WatchZoneSummary } from '../../domain/types';
import { asAuthorityId, asWatchZoneId } from '../../domain/types';

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

/** Parses the JSON body of a recorded request; asserts one was made. */
function sentBody(spy: SpyFetch): unknown {
  const call = spy.calls[0];
  if (!call) {
    throw new Error('expected a request to have been made');
  }
  return JSON.parse(call.init?.body as string);
}

const aBoundary: WatchZoneBoundary = {
  type: 'Polygon',
  coordinates: [
    [
      [0.1, 52.2],
      [0.12, 52.2],
      [0.12, 52.21],
      [0.1, 52.2],
    ],
  ],
};

function aZoneSummary(overrides?: Partial<WatchZoneSummary>): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-1'),
    name: 'Home',
    latitude: 52.2053,
    longitude: 0.1218,
    radiusMetres: 2000,
    authorityId: asAuthorityId(1),
    pushEnabled: true,
    emailInstantEnabled: true,
    paused: false,
    boundary: null,
    ...overrides,
  };
}

describe('watchZonesApi — boundary wire behaviour', () => {
  it('list() returns each zone boundary field unchanged, including null and a Polygon', async () => {
    const spy = new SpyFetch();
    const zones = [aZoneSummary({ boundary: null }), aZoneSummary({ id: asWatchZoneId('zone-2'), boundary: aBoundary })];
    spy.response = new Response(JSON.stringify({ zones }), { status: 200 });
    const api = watchZonesApi(clientWith(spy));

    const result = await api.list();

    expect(result).toEqual(zones);
  });

  it('create() sends a boundary in the POST body, preserving [lon, lat] tuple order', async () => {
    const spy = new SpyFetch();
    spy.response = new Response(JSON.stringify(aZoneSummary({ boundary: aBoundary })), { status: 201 });
    const api = watchZonesApi(clientWith(spy));

    await api.create({
      name: 'Home',
      latitude: 52.2053,
      longitude: 0.1218,
      radiusMetres: 2000,
      boundary: aBoundary,
    });

    expect(sentBody(spy)).toMatchObject({ boundary: aBoundary });
  });

  it('updateZone() omits the boundary key entirely when the caller does not set it (tri-state: leave untouched)', async () => {
    const spy = new SpyFetch();
    spy.response = new Response(JSON.stringify(aZoneSummary()), { status: 200 });
    const api = watchZonesApi(clientWith(spy));

    await api.updateZone('zone-1', { name: 'Office' });

    const body = sentBody(spy) as Record<string, unknown>;
    expect('boundary' in body).toBe(false);
  });

  it('updateZone() sends an explicit null boundary to revert a zone to a circle', async () => {
    const spy = new SpyFetch();
    spy.response = new Response(JSON.stringify(aZoneSummary()), { status: 200 });
    const api = watchZonesApi(clientWith(spy));

    await api.updateZone('zone-1', { boundary: null });

    const body = sentBody(spy) as Record<string, unknown>;
    expect('boundary' in body).toBe(true);
    expect(body['boundary']).toBeNull();
  });

  it('updateZone() sends the new boundary value when setting a shape', async () => {
    const spy = new SpyFetch();
    spy.response = new Response(JSON.stringify(aZoneSummary({ boundary: aBoundary })), { status: 200 });
    const api = watchZonesApi(clientWith(spy));

    await api.updateZone('zone-1', { boundary: aBoundary });

    expect(sentBody(spy)).toEqual({ boundary: aBoundary });
  });
});
