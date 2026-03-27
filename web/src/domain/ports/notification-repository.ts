import type { NotificationsResult } from '../types';

export interface NotificationRepository {
  list(page: number, pageSize: number): Promise<NotificationsResult>;
}
