import type { ApiClient } from '../../api/client';
import type { NotificationsResult } from '../../domain/types';
import type { NotificationRepository } from '../../domain/ports/notification-repository';
import { notificationsApi } from '../../api/notifications';

export class ApiNotificationRepository implements NotificationRepository {
  private readonly api: ReturnType<typeof notificationsApi>;

  constructor(client: ApiClient) {
    this.api = notificationsApi(client);
  }

  async list(page: number, pageSize: number): Promise<NotificationsResult> {
    return this.api.list(page, pageSize);
  }
}
