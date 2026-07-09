import { Link } from 'react-router-dom';
import type { PlanningApplicationSummary } from '../../domain/types';
import { formatDate, statusClassName, statusDisplayLabel } from '../../utils/formatting';
import { StatusIcon } from '../StatusIcon/StatusIcon';
import styles from './ApplicationCard.module.css';

interface Props {
  application: PlanningApplicationSummary;
  /**
   * Called when the card is opened (tap-to-read). Navigation still proceeds —
   * this fires alongside the link, letting the Applications list mark the
   * application read. Optional so read-only surfaces (saved list, dashboard)
   * can render the card without wiring it.
   */
  onOpen?: (application: PlanningApplicationSummary) => void;
}

const MAX_DESCRIPTION_LENGTH = 120;

function truncate(text: string | null, maxLength: number): string {
  if (text === null || text.length <= maxLength) {
    return text ?? '';
  }
  return text.slice(0, maxLength) + '...';
}

/**
 * "Filed notice" card (Public Notice component language, epic #848 R1): a
 * mono document-header strip (reference left, date right) over a 1px rule,
 * a Fraunces-set address as the card's title, an outlined status stamp
 * (icon + label — never colour alone), and a 2px top rule that is the
 * card's unread signal — text-primary when read, amber when unread. The top
 * rule replaces the former leading unread dot; it reuses the same
 * `latestUnreadEvent` signal.
 */
export function ApplicationCard({ application, onOpen }: Props) {
  const isUnread = application.latestUnreadEvent !== null;
  const statusClass = statusClassName(application.appState, styles);

  function handleClick() {
    onOpen?.(application);
  }

  return (
    <Link
      to={`/applications/${application.uid}`}
      state={{ authority: String(application.areaId), name: application.name }}
      className={`${styles.card} ${isUnread ? styles.cardUnread : styles.cardRead}`}
      data-testid="application-card"
      data-unread={isUnread ? 'true' : 'false'}
      onClick={handleClick}
    >
      <div className={styles.docHeader}>
        <span
          className={`${styles.reference} tc-mono-meta`}
          data-testid="application-reference"
        >
          {application.name}
        </span>
        {application.startDate !== null && (
          <span
            className={`${styles.metaDate} tc-mono-meta`}
            data-testid="application-start-date"
          >
            {formatDate(application.startDate)}
          </span>
        )}
      </div>

      <div className={styles.body}>
        <h3 className={styles.address}>{application.address}</h3>
        <span
          className={`${styles.statusBadge} ${statusClass}`}
          data-testid="application-status-badge"
        >
          <StatusIcon appState={application.appState} />
          {statusDisplayLabel(application.appState)}
        </span>
      </div>

      <p className={styles.description} data-testid="application-description">
        {truncate(application.description, MAX_DESCRIPTION_LENGTH)}
      </p>

      <div className={styles.meta}>
        <span className={styles.metaItem}>{application.appType}</span>
        <span className={styles.metaItem}>{application.areaName}</span>
      </div>
    </Link>
  );
}
