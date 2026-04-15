import type {
  WatchZoneSummary,
  CreateWatchZoneRequest,
  UpdateWatchZoneRequest,
  ZoneNotificationPreferences,
  UpdateZonePreferencesRequest,
} from '../types';

export interface WatchZoneRepository {
  list(): Promise<readonly WatchZoneSummary[]>;
  create(data: CreateWatchZoneRequest): Promise<WatchZoneSummary>;
  updateZone(zoneId: string, data: UpdateWatchZoneRequest): Promise<WatchZoneSummary>;
  delete(zoneId: string): Promise<void>;
  getPreferences(zoneId: string): Promise<ZoneNotificationPreferences>;
  updatePreferences(zoneId: string, data: UpdateZonePreferencesRequest): Promise<void>;
}
