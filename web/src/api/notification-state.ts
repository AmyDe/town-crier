import type { ApiClient } from './client';
import type { NotificationStateSnapshot } from '../domain/types';

interface AdvanceRequestBody {
  readonly asOf: string;
}

/**
 * HTTP client for the notification read-state endpoints. Read state is now
 * per-application (`notifications.read_at`), replacing the single last-read
 * watermark — see ADR 0035.
 *
 * The functions are named in present-imperative form to mirror their
 * underlying endpoint verbs:
 * - `getState` — `GET /v1/me/notification-state`
 * - `markAllRead` — `POST /v1/me/notification-state/mark-all-read`
 * - `markApplicationRead` — `POST /v1/me/applications/mark-read` (tap-to-read)
 */
export function notificationStateApi(client: ApiClient) {
  return {
    getState: () =>
      client.get<NotificationStateSnapshot>('/v1/me/notification-state'),
    markAllRead: () =>
      client.post<void>('/v1/me/notification-state/mark-all-read'),
    /**
     * Marks a single application's notifications read for the current user
     * (tap-to-read). The wire field `applicationUid` carries the application's
     * `name` (the PlanIt case reference, e.g. "24/0001"), NOT its `uid` — the
     * server matches it against `notifications.application_name`. The key is
     * named `applicationUid` for cross-client contract stability; do not
     * "fix" it to send the uid. `authorityId` (the app's `areaId`)
     * disambiguates refs that are only unique within a council. Idempotent
     * (204 even when zero rows match).
     */
    markApplicationRead: (applicationUid: string, authorityId: number) =>
      client.post<void>('/v1/me/applications/mark-read', {
        applications: [{ applicationUid, authorityId }],
      }),
    advance: (asOf: string) => {
      const body: AdvanceRequestBody = { asOf };
      return client.post<void>('/v1/me/notification-state/advance', body);
    },
  };
}
