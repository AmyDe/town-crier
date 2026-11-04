/** Port for the server-side notification read-state (per application). */
export interface NotificationStateRepository {
  /** Marks every unread notification for the user read. */
  markAllRead(): Promise<void>;
  /**
   * Marks one application's notifications read (tap-to-read). Idempotent.
   * `applicationUid` carries the application's `name` (PlanIt case reference),
   * NOT its `uid`. `authorityId` is the application's `areaId`.
   */
  markApplicationRead(applicationUid: string, authorityId: number): Promise<void>;
}
