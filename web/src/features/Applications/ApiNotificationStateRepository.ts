import type { ApiClient } from '../../api/client';
import type { NotificationStateSnapshot } from '../../domain/types';
import type { NotificationStateRepository } from '../../domain/ports/notification-state-repository';
import { notificationStateApi } from '../../api/notification-state';

/**
 * Concrete adapter for {@link NotificationStateRepository} backed by the
 * three watermark endpoints introduced in tc-1nsa.2. The repository is a
 * thin pass-through over {@link notificationStateApi} so the wire shape can
 * be reused by other callers (e.g. the unread-badge polling that lives
 * outside the Applications screen) without going through this class.
 */
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

  async advance(asOf: string): Promise<void> {
    return this.api.advance(asOf);
  }
}
