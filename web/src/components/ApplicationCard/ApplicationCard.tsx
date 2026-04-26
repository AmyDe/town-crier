import { Link } from 'react-router-dom';
import type { PlanningApplicationSummary } from '../../domain/types';
import { formatDate, statusClassName, statusDisplayLabel } from '../../utils/formatting';
import styles from './ApplicationCard.module.css';

interface Props {
  application: PlanningApplicationSummary;
}

const MAX_DESCRIPTION_LENGTH = 120;

function truncate(text: string | null, maxLength: number): string {
  if (text === null || text.length <= maxLength) {
    return text ?? '';
  }
  return text.slice(0, maxLength) + '...';
}

export function ApplicationCard({ application }: Props) {
  const statusClass = statusClassName(application.appState, styles);

  return (
    <Link
      to={`/applications/${application.uid}`}
      className={styles.card}
    >
      <div className={styles.header}>
        <h3 className={styles.reference}>{application.name}</h3>
        <span className={`${styles.statusBadge} ${statusClass}`}>
          {statusDisplayLabel(application.appState)}
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
