import { useState } from 'react';
import { Link } from 'react-router-dom';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import { useWatchZones } from './useWatchZones';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import { ConfirmDialog } from '../../components/ConfirmDialog/ConfirmDialog';
import styles from './WatchZoneListPage.module.css';

interface Props {
  repository: WatchZoneRepository;
}

function formatRadius(metres: number): string {
  return `${metres / 1000} km`;
}

export function WatchZoneListPage({ repository }: Props) {
  const { zones, isLoading, error, deleteZone } = useWatchZones(repository);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);

  async function handleConfirmDelete() {
    if (deleteTarget) {
      await deleteZone(deleteTarget);
      setDeleteTarget(null);
    }
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Watch Zones</h1>
        {!isLoading && zones.length > 0 && (
          <Link to="/watch-zones/new" className={styles.createButton}>
            Create Watch Zone
          </Link>
        )}
      </div>

      {isLoading && <p>Loading...</p>}

      {error && <p className={styles.error}>{error}</p>}

      {!isLoading && !error && zones.length === 0 && (
        <>
          <EmptyState
            title="No watch zones yet"
            message="Create a watch zone to start monitoring planning applications in your area."
            icon="📍"
          />
          <div className={styles.emptyAction}>
            <Link to="/watch-zones/new" className={styles.createLink}>
              Create your first watch zone
            </Link>
          </div>
        </>
      )}

      {zones.length > 0 && (
        <ul className={styles.list}>
          {zones.map((zone) => (
            <li key={zone.id} className={styles.card}>
              <div className={styles.cardContent}>
                <h2 className={styles.zoneName}>{zone.name}</h2>
                <p className={styles.zoneRadius}>{formatRadius(zone.radiusMetres)}</p>
              </div>
              <div className={styles.cardActions}>
                <Link to={`/watch-zones/${zone.id}`} className={styles.editLink}>
                  Edit
                </Link>
                <button
                  className={styles.deleteButton}
                  onClick={() => setDeleteTarget(zone.id)}
                >
                  Delete
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete Watch Zone"
        message="Are you sure you want to delete this watch zone? This action cannot be undone."
        confirmLabel="Confirm"
        onConfirm={handleConfirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
