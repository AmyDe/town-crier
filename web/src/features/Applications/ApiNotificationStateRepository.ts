import type { ApiClient } from '../../api/client';
import type { NotificationStateSnapshot } from '../../domain/types';
import type { NotificationStateRepository } from '../../domain/ports/notification-state-repository';
import { notificationStateApi } from '../../api/notification-state';

/** {@link NotificationStateRepository} over {@link notificationStateApi}. */
export class ApiNotificationStateRepository
  implements NotificationStateRepository
{
  private readonly api: ReturnType<typeof notificationStateApi>;

  constructor(client: ApiClient) {
    this.api = notificationStateApi(client);
  }

  async getState(): Promise<NotificationStateSnapshot> {
    return this.api.getState();
  }

  async markAllRead(): Promise<void> {
    return this.api.markAllRead();
  }

  async markApplicationRead(
    applicationUid: string,
    authorityId: number,
  ): Promise<void> {
    return this.api.markApplicationRead(applicationUid, authorityId);
  }
}
