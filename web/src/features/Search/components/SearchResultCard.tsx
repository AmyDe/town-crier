import { useState } from 'react';
import type { SearchResult } from '../../../domain/types';
import { buildShareUrl } from '../../../domain/share-link';
import { formatDate, statusClassName, statusDisplayLabel } from '../../../utils/formatting';
import { StatusIcon } from '../../../components/StatusIcon/StatusIcon';
import styles from './SearchResultCard.module.css';

interface Props {
  result: SearchResult;
}

type CopyState = 'idle' | 'copied' | 'error';

/**
 * One row of `/search` results: an application summary, a link to its public
 * share page, and the copy-to-clipboard button that is this bead's core
 * acceptance criterion (#821 Phase 4).
 */
export function SearchResultCard({ result }: Props) {
  const [copyState, setCopyState] = useState<CopyState>('idle');
  const shareUrl = buildShareUrl(result.authoritySlug, result.reference);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopyState('copied');
    } catch {
      setCopyState('error');
    }
  }

  return (
    <li className={styles.card}>
      <div className={styles.header}>
        <h3 className={styles.reference}>{result.reference}</h3>
        {result.appState !== null && (
          <span className={`${styles.statusBadge} ${statusClassName(result.appState, styles)}`}>
            <StatusIcon appState={result.appState} />
            {statusDisplayLabel(result.appState)}
          </span>
        )}
      </div>

      <p className={styles.address}>{result.address}</p>
      <p className={styles.authority}>{result.authorityName}</p>

      {(result.startDate !== null || result.decidedDate !== null) && (
        <div className={styles.meta}>
          {result.startDate !== null && <span>Received {formatDate(result.startDate)}</span>}
          {result.decidedDate !== null && <span>Decided {formatDate(result.decidedDate)}</span>}
        </div>
      )}

      <div className={styles.actions}>
        <a href={shareUrl} className={styles.shareLink} target="_blank" rel="noopener noreferrer">
          View share page
        </a>
        <button type="button" className={styles.copyButton} onClick={() => void handleCopy()}>
          {copyState === 'copied' ? 'Copied!' : 'Copy link'}
        </button>
        <span aria-live="polite" className={styles.copyStatus}>
          {copyState === 'copied' && 'Share link copied to clipboard'}
          {copyState === 'error' && "Couldn't copy the link. Try selecting it from the address bar."}
        </span>
      </div>
    </li>
  );
}
