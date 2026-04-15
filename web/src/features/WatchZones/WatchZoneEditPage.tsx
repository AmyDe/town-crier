import { Link } from 'react-router-dom';
import type { WatchZoneSummary } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import { useZonePreferences } from './useZonePreferences';
import { useZoneEdit } from './useZoneEdit';
import { RadiusPicker } from '../../components/RadiusPicker/RadiusPicker';
import styles from './WatchZoneEditPage.module.css';

interface Props {
  repository: WatchZoneRepository;
  zone: WatchZoneSummary;
}

export function WatchZoneEditPage({ repository, zone }: Props) {
  const { preferences, isLoading, error, updatePreferences } = useZonePreferences(
    repository,
    zone.id,
  );

  const zoneEdit = useZoneEdit(repository, zone);

  function handleToggle(field: 'newApplications' | 'statusChanges' | 'decisionUpdates') {
    if (!preferences) return;
    updatePreferences({
      newApplications: field === 'newApplications' ? !preferences.newApplications : preferences.newApplications,
      statusChanges: field === 'statusChanges' ? !preferences.statusChanges : preferences.statusChanges,
      decisionUpdates: field === 'decisionUpdates' ? !preferences.decisionUpdates : preferences.decisionUpdates,
    });
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <Link to="/watch-zones" className={styles.backLink}>
          Back to Watch Zones
        </Link>
      </div>

      <section className={styles.zonePropertiesSection}>
        <h2 className={styles.sectionTitle}>Zone Details</h2>

        <div className={styles.field}>
          <label htmlFor="zone-name" className={styles.fieldLabel}>
            Zone name
          </label>
          <input
            id="zone-name"
            type="text"
            className={styles.textInput}
            value={zoneEdit.name}
            onChange={(e) => zoneEdit.setName(e.target.value)}
            aria-label="Zone name"
          />
          {zoneEdit.nameError && (
            <p className={styles.fieldError}>{zoneEdit.nameError}</p>
          )}
        </div>

        <RadiusPicker
          selectedMetres={zoneEdit.radiusMetres}
          onSelect={zoneEdit.setRadiusMetres}
        />

        {zoneEdit.isDirty && (
          <button
            type="button"
            className={styles.saveButton}
            onClick={zoneEdit.save}
            disabled={!zoneEdit.canSave || zoneEdit.isSaving}
            aria-label="Save zone changes"
          >
            {zoneEdit.isSaving ? 'Saving...' : 'Save'}
          </button>
        )}

        {zoneEdit.error && (
          <p className={styles.error}>{zoneEdit.error}</p>
        )}
      </section>

      {isLoading && <p className={styles.loading}>Loading...</p>}

      {error && <p className={styles.error}>{error}</p>}

      {preferences && (
        <section className={styles.preferencesSection}>
          <h2 className={styles.sectionTitle}>Notification Preferences</h2>

          <label className={styles.toggle}>
            <input
              type="checkbox"
              checked={preferences.newApplications}
              onChange={() => handleToggle('newApplications')}
              aria-label="New applications"
            />
            <span className={styles.toggleLabel}>New applications</span>
          </label>

          <label className={styles.toggle}>
            <input
              type="checkbox"
              checked={preferences.statusChanges}
              onChange={() => handleToggle('statusChanges')}
              aria-label="Status changes"
            />
            <span className={styles.toggleLabel}>Status changes</span>
          </label>

          <label className={styles.toggle}>
            <input
              type="checkbox"
              checked={preferences.decisionUpdates}
              onChange={() => handleToggle('decisionUpdates')}
              aria-label="Decision updates"
            />
            <span className={styles.toggleLabel}>Decision updates</span>
          </label>
        </section>
      )}
    </div>
  );
}
