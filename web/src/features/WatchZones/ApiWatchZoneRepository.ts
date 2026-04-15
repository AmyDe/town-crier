import type { ApiClient } from '../../api/client';
import type {
  WatchZoneSummary,
  CreateWatchZoneRequest,
  UpdateWatchZoneRequest,
  ZoneNotificationPreferences,
  UpdateZonePreferencesRequest,
} from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import { watchZonesApi } from '../../api/watchZones';

export class ApiWatchZoneRepository implements WatchZoneRepository {
  private readonly api: ReturnType<typeof watchZonesApi>;

  constructor(client: ApiClient) {
    this.api = watchZonesApi(client);
  }

  async list(): Promise<readonly WatchZoneSummary[]> {
    return this.api.list();
  }

  async create(data: CreateWatchZoneRequest): Promise<WatchZoneSummary> {
    return this.api.create(data);
  }

  async updateZone(zoneId: string, data: UpdateWatchZoneRequest): Promise<WatchZoneSummary> {
    return this.api.updateZone(zoneId, data);
  }

  async delete(zoneId: string): Promise<void> {
    return this.api.delete(zoneId);
  }

  async getPreferences(zoneId: string): Promise<ZoneNotificationPreferences> {
    return this.api.getPreferences(zoneId);
  }

  async updatePreferences(zoneId: string, data: UpdateZonePreferencesRequest): Promise<void> {
    return this.api.updatePreferences(zoneId, data);
  }
}
