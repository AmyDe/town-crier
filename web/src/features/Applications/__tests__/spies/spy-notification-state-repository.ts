import type { NotificationStateRepository } from '../../../../domain/ports/notification-state-repository';

export class SpyNotificationStateRepository
  implements NotificationStateRepository
{
  markAllReadCalls = 0;

  markApplicationReadCalls: Array<{ applicationUid: string; authorityId: number }> = [];

  async markAllRead(): Promise<void> {
    this.markAllReadCalls++;
  }

  async markApplicationRead(
    applicationUid: string,
    authorityId: number,
  ): Promise<void> {
    this.markApplicationReadCalls.push({ applicationUid, authorityId });
  }
}
