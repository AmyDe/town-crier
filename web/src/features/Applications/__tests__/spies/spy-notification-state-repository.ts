import type { NotificationStateSnapshot } from '../../../../domain/types';
import type { NotificationStateRepository } from '../../../../domain/ports/notification-state-repository';

/**
 * Hand-written spy for the notification-state port. Lets tests assert that
 * `useApplications` (and any other consumer) hits the watermark API in the
 * expected order without reaching for `vi.fn()`/`vi.mock()`.
 */
export class SpyNotificationStateRepository
  implements NotificationStateRepository
{
  getStateCalls = 0;
  getStateResult: NotificationStateSnapshot = {
    lastReadAt: '2026-01-01T00:00:00Z',
    version: 1,
    totalUnreadCount: 0,
  };
  getStateError: Error | null = null;

  markAllReadCalls = 0;
  markAllReadError: Error | null = null;

  advanceCalls: string[] = [];
  advanceError: Error | null = null;

  async getState(): Promise<NotificationStateSnapshot> {
    this.getStateCalls++;
    if (this.getStateError) {
      throw this.getStateError;
    }
    return this.getStateResult;
  }

  async markAllRead(): Promise<void> {
    this.markAllReadCalls++;
    if (this.markAllReadError) {
      throw this.markAllReadError;
    }
  }

  async advance(asOf: string): Promise<void> {
    this.advanceCalls.push(asOf);
    if (this.advanceError) {
      throw this.advanceError;
    }
  }
}
