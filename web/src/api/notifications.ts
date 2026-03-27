import type { ApiClient } from './client';
import type { NotificationsResult } from '../domain/types';

export function notificationsApi(client: ApiClient) {
  return {
    list: (page: number = 1, pageSize: number = 20) =>
      client.get<NotificationsResult>('/v1/notifications', {
        page: String(page),
        pageSize: String(pageSize),
      }),
  };
}
