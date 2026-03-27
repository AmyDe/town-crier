import type { NotificationItem, NotificationsResult } from '../../../../domain/types';
import { asAuthorityId } from '../../../../domain/types';

export function aNotification(overrides?: Partial<NotificationItem>): NotificationItem {
  return {
    applicationName: '2026/0042',
    applicationAddress: '12 Mill Road, Cambridge, CB1 2AD',
    applicationDescription: 'Erection of two-storey rear extension',
    applicationType: 'Full',
    authorityId: asAuthorityId(1),
    createdAt: '2026-03-15T10:30:00Z',
    ...overrides,
  };
}

export function aSecondNotification(overrides?: Partial<NotificationItem>): NotificationItem {
  return {
    applicationName: '2026/0099',
    applicationAddress: '45 High Street, Cambridge, CB2 1LA',
    applicationDescription: 'Change of use from retail to residential',
    applicationType: 'Change of Use',
    authorityId: asAuthorityId(2),
    createdAt: '2026-03-14T14:00:00Z',
    ...overrides,
  };
}

export function notificationsPage(
  items: NotificationItem[] = [aNotification(), aSecondNotification()],
  total: number = 2,
  page: number = 1,
): NotificationsResult {
  return { notifications: items, total, page };
}
