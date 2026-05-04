import type { ApiClient } from './client';
import type { NotificationStateSnapshot } from '../domain/types';

interface AdvanceRequestBody {
  readonly asOf: string;
}

/**
 * HTTP client for the three notification-state endpoints introduced by
 * tc-1nsa.2. See spec
 * `docs/specs/notifications-unread-watermark.md#api-surface`.
 *
 * The functions are named in present-imperative form to mirror their
 * underlying endpoint verbs:
 * - `getState` — `GET /v1/me/notification-state`
 * - `markAllRead` — `POST /v1/me/notification-state/mark-all-read`
 * - `advance` — `POST /v1/me/notification-state/advance`
 */
export function notificationStateApi(client: ApiClient) {
  return {
    getState: () =>
      client.get<NotificationStateSnapshot>('/v1/me/notification-state'),
    markAllRead: () =>
      client.post<void>('/v1/me/notification-state/mark-all-read'),
    advance: (asOf: string) => {
      const body: AdvanceRequestBody = { asOf };
      return client.post<void>('/v1/me/notification-state/advance', body);
    },
  };
}
