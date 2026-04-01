import type { ApiClient } from './client';
import type {
  WatchZoneSummary,
  CreateWatchZoneRequest,
  ZoneNotificationPreferences,
  UpdateZonePreferencesRequest,
} from '../domain/types';

export function watchZonesApi(client: ApiClient) {
  return {
    list: async () => {
      const result = await client.get<{ zones: readonly WatchZoneSummary[] }>('/v1/me/watch-zones');
      return result.zones;
    },
    create: (data: CreateWatchZoneRequest) =>
      client.post<WatchZoneSummary>('/v1/me/watch-zones', data),
    delete: (zoneId: string) => client.delete(`/v1/me/watch-zones/${zoneId}`),
    getPreferences: (zoneId: string) =>
      client.get<ZoneNotificationPreferences>(`/v1/me/watch-zones/${zoneId}/preferences`),
    updatePreferences: (zoneId: string, data: UpdateZonePreferencesRequest) =>
      client.put(`/v1/me/watch-zones/${zoneId}/preferences`, data),
  };
}
