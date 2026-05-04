import { Link } from 'react-router-dom';
import type { WatchZoneSummary, SubscriptionTier } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import { useZonePreferences } from './useZonePreferences';
import { useZoneEdit } from './useZoneEdit';
import { RadiusPicker } from '../../components/RadiusPicker/RadiusPicker';
import { LargeRadiusWarning } from '../../components/LargeRadiusWarning/LargeRadiusWarning';
import { Toggle } from '../../components/Toggle/Toggle';
import styles from './WatchZoneEditPage.module.css';

interface Props {
  repository: WatchZoneRepository;
  zone: WatchZoneSummary;
  /**
   * The current user's subscription tier. Per-zone push and instant-email
   * toggles are only rendered for Personal/Pro tiers — Free users see no
   * notification controls (they fall back to the account-level weekly digest).
   * Optional so legacy call sites keep compiling; defaults to Free, which
   * hides the toggles.
   */
  tier?: SubscriptionTier;
}

export function WatchZoneEditPage({ repository, zone, tier = 'Free' }: Props) {
  const { preferences, isLoading, error, updatePreferences } = useZonePreferences(
    repository,
    zone.id,
  );

  const zoneEdit = useZoneEdit(repository, zone);
  const showZoneNotificationToggles = tier !== 'Free';

  type PreferenceField =
    | 'newApplicationPush'
    | 'newApplicationEmail'
    | 'decisionPush'
    | 'decisionEmail';

  function handleToggle(field: PreferenceField) {
    if (!preferences) return;
    updatePreferences({
      newApplicationPush:
        field === 'newApplicationPush'
          ? !preferences.newApplicationPush
          : preferences.newApplicationPush,
      newApplicationEmail:
        field === 'newApplicationEmail'
          ? !preferences.newApplicationEmail
          : preferences.newApplicationEmail,
      decisionPush:
        field === 'decisionPush' ? !preferences.decisionPush : preferences.decisionPush,
      decisionEmail:
        field === 'decisionEmail' ? !preferences.decisionEmail : preferences.decisionEmail,
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

        <LargeRadiusWarning radiusMetres={zoneEdit.radiusMetres} />

        {showZoneNotificationToggles && (
          <div className={styles.zoneNotifications}>
            <div className={styles.toggleRow}>
              <span className={styles.toggleRowLabel}>Push notifications</span>
              <Toggle
                checked={zoneEdit.pushEnabled}
                onChange={zoneEdit.setPushEnabled}
                label="Push notifications"
              />
            </div>
            <div className={styles.toggleRow}>
              <span className={styles.toggleRowLabel}>Instant emails</span>
              <Toggle
                checked={zoneEdit.emailInstantEnabled}
                onChange={zoneEdit.setEmailInstantEnabled}
                label="Instant emails"
              />
            </div>
          </div>
        )}

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

          <h3 className={styles.preferencesGroupTitle}>New applications</h3>
          <label className={styles.toggle}>
            <input
              type="checkbox"
              checked={preferences.newApplicationPush}
              onChange={() => handleToggle('newApplicationPush')}
              aria-label="New applications — push"
            />
            <span className={styles.toggleLabel}>Push</span>
          </label>

          <label className={styles.toggle}>
            <input
              type="checkbox"
              checked={preferences.newApplicationEmail}
              onChange={() => handleToggle('newApplicationEmail')}
              aria-label="New applications — email"
            />
            <span className={styles.toggleLabel}>Email</span>
          </label>

          <h3 className={styles.preferencesGroupTitle}>Decision updates</h3>
          <label className={styles.toggle}>
            <input
              type="checkbox"
              checked={preferences.decisionPush}
              onChange={() => handleToggle('decisionPush')}
              aria-label="Decision updates — push"
            />
            <span className={styles.toggleLabel}>Push</span>
          </label>

          <label className={styles.toggle}>
            <input
              type="checkbox"
              checked={preferences.decisionEmail}
              onChange={() => handleToggle('decisionEmail')}
              aria-label="Decision updates — email"
            />
            <span className={styles.toggleLabel}>Email</span>
          </label>
        </section>
      )}
    </div>
  );
}
