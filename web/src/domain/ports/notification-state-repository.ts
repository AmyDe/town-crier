import type { NotificationStateSnapshot } from '../types';

/** Port for the server-side notification read-state (per application). */
export interface NotificationStateRepository {
  /** Returns the user's current read-state snapshot and unread count. */
  getState(): Promise<NotificationStateSnapshot>;
  /** Marks every unread notification for the user read. */
  markAllRead(): Promise<void>;
  /**
   * Marks one application's notifications read (tap-to-read). Idempotent.
   * `applicationUid` carries the application's `name` (PlanIt case reference),
   * NOT its `uid`. `authorityId` is the application's `areaId`.
   */
  markApplicationRead(applicationUid: string, authorityId: number): Promise<void>;
}
