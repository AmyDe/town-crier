import type { NotificationStateSnapshot } from '../types';

/**
 * Port for the server-side notification read-state. Read state is per
 * application (`notifications.read_at`), replacing the single last-read
 * watermark — see ADR 0035.
 *
 * Backed by these endpoints:
 * - `GET /v1/me/notification-state` returns the current snapshot.
 * - `POST /v1/me/notification-state/mark-all-read` clears every unread
 *   notification for the user; subsequent fetches report zero unread.
 * - `POST /v1/me/applications/mark-read` clears the unread notifications for a
 *   single application (tap-to-read).
 */
export interface NotificationStateRepository {
  /** Returns the user's current read-state snapshot and unread count. */
  getState(): Promise<NotificationStateSnapshot>;
  /** Marks every unread notification for the user read. */
  markAllRead(): Promise<void>;
  /**
   * Marks a single application's notifications read (tap-to-read). Idempotent —
   * a second call for an already-read application is a no-op server-side.
   *
   * `applicationUid` carries the application's `name` (the PlanIt case
   * reference), NOT its `uid` — the wire key is named `applicationUid` for
   * cross-client contract stability but the value is the reference. See
   * {@link notificationStateApi} for the full contract note. `authorityId` is
   * the application's `areaId`, needed because refs are only unique within a
   * council.
   */
  markApplicationRead(applicationUid: string, authorityId: number): Promise<void>;
  /**
   * Advances the watermark forward to `asOf`. No-op if `asOf` is at or before
   * the existing watermark (server-enforced monotonicity).
   */
  advance(asOf: string): Promise<void>;
}
