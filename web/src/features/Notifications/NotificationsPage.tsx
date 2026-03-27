import type { NotificationRepository } from '../../domain/ports/notification-repository';
import { useNotifications } from './useNotifications';
import { Pagination } from '../../components/Pagination/Pagination';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import styles from './NotificationsPage.module.css';

interface Props {
  repository: NotificationRepository;
}

function formatTimestamp(iso: string): string {
  const date = new Date(iso);
  return date.toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  });
}

export function NotificationsPage({ repository }: Props) {
  const {
    notifications,
    page,
    totalPages,
    isLoading,
    error,
    goToNextPage,
    goToPreviousPage,
  } = useNotifications(repository);

  if (isLoading) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Notifications</h1>
        <p className={styles.loading}>Loading notifications...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Notifications</h1>
        <p className={styles.error}>{error}</p>
      </div>
    );
  }

  if (notifications.length === 0) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Notifications</h1>
        <EmptyState
          icon="🔔"
          title="No notifications"
          message="You'll see notifications here when there are updates to applications in your watch zones."
        />
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Notifications</h1>
      <ul className={styles.list}>
        {notifications.map((notification, index) => (
          <li key={`${notification.applicationName}-${notification.createdAt}-${index}`} className={styles.item}>
            <div className={styles.itemHeader}>
              <h3 className={styles.appName}>{notification.applicationName}</h3>
              <span className={styles.typeBadge}>{notification.applicationType}</span>
            </div>
            <p className={styles.address}>{notification.applicationAddress}</p>
            <p className={styles.description}>{notification.applicationDescription}</p>
            <span className={styles.timestamp}>{formatTimestamp(notification.createdAt)}</span>
          </li>
        ))}
      </ul>
      <div className={styles.paginationWrapper}>
        <Pagination
          page={page}
          totalPages={totalPages}
          onNext={goToNextPage}
          onPrevious={goToPreviousPage}
        />
      </div>
    </div>
  );
}
