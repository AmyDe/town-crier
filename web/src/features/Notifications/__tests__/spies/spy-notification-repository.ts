import type { NotificationsResult } from '../../../../domain/types';
import type { NotificationRepository } from '../../../../domain/ports/notification-repository';

export class SpyNotificationRepository implements NotificationRepository {
  listCalls: Array<{ page: number; pageSize: number }> = [];
  listResult: NotificationsResult = { notifications: [], total: 0, page: 1 };
  listError: Error | null = null;

  async list(page: number, pageSize: number): Promise<NotificationsResult> {
    this.listCalls.push({ page, pageSize });
    if (this.listError) {
      throw this.listError;
    }
    return this.listResult;
  }
}
