import { useState } from 'react';
import type { ApplicationUid } from '../../domain/types';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
import { useSavedApplications } from './useSavedApplications';
import { ApplicationCard } from '../../components/ApplicationCard/ApplicationCard';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import { ConfirmDialog } from '../../components/ConfirmDialog/ConfirmDialog';
import styles from './SavedApplicationsPage.module.css';

interface Props {
  repository: SavedApplicationRepository;
}

export function SavedApplicationsPage({ repository }: Props) {
  const { savedApplications, isLoading, error, remove } =
    useSavedApplications(repository);

  const [pendingRemoveUid, setPendingRemoveUid] = useState<ApplicationUid | null>(null);

  function handleRemoveClick(uid: ApplicationUid) {
    setPendingRemoveUid(uid);
  }

  function handleConfirmRemove() {
    if (pendingRemoveUid) {
      remove(pendingRemoveUid);
      setPendingRemoveUid(null);
    }
  }

  function handleCancelRemove() {
    setPendingRemoveUid(null);
  }

  if (isLoading) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Saved Applications</h1>
        <p className={styles.loading}>Loading saved applications...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Saved Applications</h1>
        <p className={styles.error}>{error}</p>
      </div>
    );
  }

  if (savedApplications.length === 0) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Saved Applications</h1>
        <EmptyState
          icon="📌"
          title="No saved applications"
          message="Applications you save will appear here for quick access."
        />
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Saved Applications</h1>
      <ul className={styles.list}>
        {savedApplications.map(saved => (
          <li key={saved.applicationUid} className={styles.item}>
            <ApplicationCard application={saved.application} />
            <button
              className={styles.removeButton}
              onClick={(e) => {
                e.preventDefault();
                handleRemoveClick(saved.applicationUid);
              }}
              aria-label={`Remove ${saved.application.name}`}
            >
              ×
            </button>
          </li>
        ))}
      </ul>
      <ConfirmDialog
        open={pendingRemoveUid !== null}
        title="Remove saved application"
        message="Are you sure you want to remove this application from your saved list?"
        confirmLabel="Remove"
        onConfirm={handleConfirmRemove}
        onCancel={handleCancelRemove}
      />
    </div>
  );
}
