import { describe, it, expect, beforeEach } from 'vitest';
import type { ApiClient } from '../../../api/client';
import { ApiWatchZoneRepository } from '../ApiWatchZoneRepository';
import { aWatchZone } from './fixtures/watch-zone.fixtures';

class StubApiClient implements ApiClient {
  lastPatchPath: string | null = null;
  lastPatchBody: unknown = null;
  patchResult: unknown = null;

  async get<T>(): Promise<T> {
    return {} as T;
  }

  async post<T>(): Promise<T> {
    return {} as T;
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
});
