import { Link } from 'react-router-dom';
import type { PlanningApplicationSummary } from '../../domain/types';
import styles from './ApplicationCard.module.css';

interface Props {
  application: PlanningApplicationSummary;
}

const MAX_DESCRIPTION_LENGTH = 120;

function statusClassName(appState: string): string {
  switch (appState) {
    case 'Undecided':
      return styles.statusUndecided ?? '';
    case 'Approved':
      return styles.statusApproved ?? '';
    case 'Refused':
      return styles.statusRefused ?? '';
    case 'Withdrawn':
      return styles.statusWithdrawn ?? '';
    case 'Appealed':
      return styles.statusAppealed ?? '';
    default:
      return styles.statusDefault ?? '';
  }
}

function truncate(text: string, maxLength: number): string {
  if (text.length <= maxLength) {
    return text;
  }
  return text.slice(0, maxLength) + '...';
}

function formatDate(isoDate: string): string {
  const date = new Date(isoDate);
  return date.toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  });
}

export function ApplicationCard({ application }: Props) {
  const statusClass = statusClassName(application.appState);

  return (
    <Link
      to={`/applications/${application.uid}`}
      className={styles.card}
    >
      <div className={styles.header}>
        <h3 className={styles.reference}>{application.name}</h3>
        <span className={`${styles.statusBadge} ${statusClass}`}>
          {application.appState}
        </span>
      </div>

      <p className={styles.address}>{application.address}</p>

      <p className={styles.description} data-testid="application-description">
        {truncate(application.description, MAX_DESCRIPTION_LENGTH)}
      </p>

      <div className={styles.meta}>
        <span className={styles.metaItem}>{application.appType}</span>
        <span className={styles.metaItem}>{application.areaName}</span>
        {application.startDate !== null && (
          <span className={styles.metaItem} data-testid="application-start-date">
            {formatDate(application.startDate)}
          </span>
        )}
      </div>
    </Link>
  );
}
