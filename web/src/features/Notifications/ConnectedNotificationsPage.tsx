import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { ApiNotificationRepository } from './ApiNotificationRepository';
import { NotificationsPage } from './NotificationsPage';

export function ConnectedNotificationsPage() {
  const client = useApiClient();
  const repository = useMemo(() => new ApiNotificationRepository(client), [client]);

  return <NotificationsPage repository={repository} />;
}
