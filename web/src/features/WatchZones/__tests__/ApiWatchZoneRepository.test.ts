import { describe, it, expect, beforeEach } from 'vitest';
import type { ApiClient } from '../../../api/client';
import { ApiWatchZoneRepository } from '../ApiWatchZoneRepository';
import { aWatchZone, aWatchZoneBoundary } from './fixtures/watch-zone.fixtures';

class StubApiClient implements ApiClient {
  lastGetPath: string | null = null;
  getResult: unknown = null;

  lastPostPath: string | null = null;
  lastPostBody: unknown = null;
  postResult: unknown = null;

  lastPatchPath: string | null = null;
  lastPatchBody: unknown = null;
  patchResult: unknown = null;

  async get<T>(path: string): Promise<T> {
    this.lastGetPath = path;
    return this.getResult as T;
  }

  async getWithHeaders<T>(): Promise<{ body: T; headers: Headers }> {
    return { body: {} as T, headers: new Headers() };
  }

  async post<T>(path: string, body?: unknown): Promise<T> {
    this.lastPostPath = path;
    this.lastPostBody = body;
    return this.postResult as T;
  }

  async put(): Promise<void> {
    // no-op
  }

  async patch<T>(path: string, body?: unknown): Promise<T> {
    this.lastPatchPath = path;
    this.lastPatchBody = body;
    return this.patchResult as T;
  }

  async delete(): Promise<void> {
    // no-op
  }
}

describe('ApiWatchZoneRepository', () => {
  let stub: StubApiClient;
  let repository: ApiWatchZoneRepository;

  beforeEach(() => {
    stub = new StubApiClient();
    repository = new ApiWatchZoneRepository(stub);
  });

  it('calls PATCH /v1/me/watch-zones/{zoneId} with the update data', async () => {
    const updated = aWatchZone({ name: 'Office' });
    stub.patchResult = updated;

    const result = await repository.updateZone('zone-1', { name: 'Office' });

    expect(stub.lastPatchPath).toBe('/v1/me/watch-zones/zone-1');
    expect(stub.lastPatchBody).toEqual({ name: 'Office' });
    expect(result).toEqual(updated);
  });

  describe('boundary field', () => {
    it('list() returns each zone with its boundary field, null and Polygon alike', async () => {
      const circleZone = aWatchZone({ boundary: null });
      const customShapeZone = aWatchZone({
        id: circleZone.id,
        name: 'Office',
        boundary: aWatchZoneBoundary(),
      });
      stub.getResult = { zones: [circleZone, customShapeZone] };

      const result = await repository.list();

      expect(result).toEqual([circleZone, customShapeZone]);
    });

    it('create() forwards a boundary in the request and returns the created zone unchanged', async () => {
      const boundary = aWatchZoneBoundary();
      const created = aWatchZone({ boundary });
      stub.postResult = created;

      const result = await repository.create({
        name: 'Home',
        latitude: 52.2053,
        longitude: 0.1218,
        radiusMetres: 2000,
        boundary,
      });

      expect(stub.lastPostPath).toBe('/v1/me/watch-zones');
      expect(stub.lastPostBody).toMatchObject({ boundary });
      expect(result).toEqual(created);
    });

    it('create() omits boundary from the request for a plain circle create', async () => {
      stub.postResult = aWatchZone();

      await repository.create({
        name: 'Home',
        latitude: 52.2053,
        longitude: 0.1218,
        radiusMetres: 2000,
      });

      const body = stub.lastPostBody as Record<string, unknown>;
      expect('boundary' in body).toBe(false);
    });

    it('updateZone() forwards the update data without adding a boundary key when the caller omits it', async () => {
      stub.patchResult = aWatchZone({ name: 'Office' });

      await repository.updateZone('zone-1', { name: 'Office' });

      const body = stub.lastPatchBody as Record<string, unknown>;
      expect('boundary' in body).toBe(false);
    });

    it('updateZone() passes an explicit null boundary through unchanged (revert to circle)', async () => {
      stub.patchResult = aWatchZone({ boundary: null });

      await repository.updateZone('zone-1', { boundary: null });

      expect(stub.lastPatchBody).toEqual({ boundary: null });
    });

    it('updateZone() passes a new boundary value through unchanged (set/replace shape)', async () => {
      const boundary = aWatchZoneBoundary();
      stub.patchResult = aWatchZone({ boundary });

      await repository.updateZone('zone-1', { boundary });

      expect(stub.lastPatchBody).toEqual({ boundary });
    });
  });
});
