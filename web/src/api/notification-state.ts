import type { ApiClient } from './client';
import type { NotificationStateSnapshot } from '../domain/types';

/** HTTP client for the notification read-state endpoints. */
export function notificationStateApi(client: ApiClient) {
  return {
    getState: () =>
      client.get<NotificationStateSnapshot>('/v1/me/notification-state'),
    markAllRead: () =>
      client.post<void>('/v1/me/notification-state/mark-all-read'),
    /**
     * Marks one application's notifications read (tap-to-read). The wire field
     * `applicationUid` carries the application's `name` (PlanIt case reference),
     * NOT its `uid`; do not "fix" it. `authorityId` is the app's `areaId`.
     * Idempotent (204 even when zero rows match).
     */
    markApplicationRead: (applicationUid: string, authorityId: number) =>
      client.post<void>('/v1/me/applications/mark-read', {
        applications: [{ applicationUid, authorityId }],
      }),
  };
}
