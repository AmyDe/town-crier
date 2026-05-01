import { useCallback, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../../auth/auth-context';
import { useTheme } from '../../hooks/useTheme';
import { useUserProfile } from './useUserProfile';
import { ThemeToggle } from '../../components/ThemeToggle/ThemeToggle';
import { ConfirmDialog } from '../../components/ConfirmDialog/ConfirmDialog';
import { Toggle } from '../../components/Toggle/Toggle';
import { DAY_OF_WEEK_LABELS, type DayOfWeek } from '../../domain/types';
import type { SettingsRepository } from '../../domain/ports/settings-repository';
import { RedeemOfferCode } from '../offerCode/RedeemOfferCode';
import type { RedeemOfferCodeClient } from '../offerCode/api/redeemOfferCode';
import type { RedeemResult } from '../offerCode/api/types';
import styles from './SettingsPage.module.css';

interface Props {
  repository: SettingsRepository;
  /**
   * Optional offer-code redemption client. When provided, a "Redeem offer
   * code" section is rendered under the profile/subscription summary.
   */
  redeemOfferCodeClient?: RedeemOfferCodeClient;
  /**
   * Side-effect callback invoked on a successful redemption. Consumers use
   * this to refresh the Auth0 access token (`getAccessTokenSilently({
   * cacheMode: 'off' })`) so the new subscription claim is picked up.
   * The Settings page always re-fetches its own profile query on success.
   */
  onRedeemSuccess?: (result: RedeemResult) => void;
}

export function SettingsPage({ repository, redeemOfferCodeClient, onRedeemSuccess }: Props) {
  const { logout } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const {
    profile,
    isLoading,
    isExporting,
    isDeleting,
    error,
    exportData,
    deleteAccount,
    updatePreferences,
    refresh,
  } = useUserProfile(repository, () => logout());

  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  const handleRedeemSuccess = useCallback(
    (result: RedeemResult) => {
      // Re-fetch the profile so the tier summary reflects the new entitlement.
      refresh();
      onRedeemSuccess?.(result);
    },
    [refresh, onRedeemSuccess],
  );

  if (isLoading) {
    return (
      <div className={styles.container}>
        <p className={styles.loading}>Loading...</p>
      </div>
    );
  }

  if (error && !profile) {
    return (
      <div className={styles.container}>
        <h1 className={styles.pageTitle}>Settings</h1>
        <p className={styles.error}>{error}</p>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.pageTitle}>Settings</h1>

      {/* Profile section */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Profile</h2>
        <div className={styles.card}>
          <div className={styles.field}>
            <span className={styles.label}>User ID</span>
            <span className={styles.value}>{profile?.userId}</span>
          </div>
          <div className={styles.field}>
            <span className={styles.label}>Subscription</span>
            <span className={styles.value}>{profile?.tier}</span>
          </div>
        </div>
      </section>

      {/* Redeem offer code section (only rendered when a client is wired) */}
      {redeemOfferCodeClient && (
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Redeem offer code</h2>
          <div className={styles.card}>
            <RedeemOfferCode
              client={redeemOfferCodeClient}
              onSuccess={handleRedeemSuccess}
            />
          </div>
        </section>
      )}

      {/* Notifications section */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Notifications</h2>
        <div className={styles.card}>
          <div className={styles.toggleRow}>
            <span className={styles.label}>Email digest</span>
            <Toggle
              checked={profile?.emailDigestEnabled ?? true}
              onChange={(checked) => updatePreferences({ emailDigestEnabled: checked })}
              label="Email digest"
            />
          </div>
          {profile?.emailDigestEnabled && (
            <div className={styles.selectRow}>
              <label htmlFor="digest-day" className={styles.label}>Digest day</label>
              <select
                id="digest-day"
                className={styles.select}
                value={profile.digestDay}
                onChange={(e) => updatePreferences({ digestDay: Number(e.target.value) as DayOfWeek })}
              >
                {([1, 2, 3, 4, 5, 6, 0] as const).map((day) => (
                  <option key={day} value={day}>
                    {DAY_OF_WEEK_LABELS[day]}
                  </option>
                ))}
              </select>
            </div>
          )}

          <h3 className={styles.preferencesGroupTitle}>Saved applications</h3>
          <div className={styles.toggleRow}>
            <span className={styles.label}>Push</span>
            <Toggle
              checked={profile?.savedDecisionPush ?? true}
              onChange={(checked) => updatePreferences({ savedDecisionPush: checked })}
              label="Saved applications — push"
            />
          </div>
          <div className={styles.toggleRow}>
            <span className={styles.label}>Email</span>
            <Toggle
              checked={profile?.savedDecisionEmail ?? true}
              onChange={(checked) => updatePreferences({ savedDecisionEmail: checked })}
              label="Saved applications — email"
            />
          </div>
        </div>
      </section>

      {/* Appearance section */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Appearance</h2>
        <div className={styles.card}>
          <div className={styles.themeRow}>
            <span className={styles.label}>Theme</span>
            <ThemeToggle theme={theme} toggleTheme={toggleTheme} />
          </div>
        </div>
      </section>

      {/* Data section */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Data</h2>
        <div className={styles.card}>
          <button
            className={styles.secondaryButton}
            onClick={exportData}
            disabled={isExporting}
          >
            {isExporting ? 'Exporting...' : 'Export Your Data'}
          </button>
        </div>
      </section>

      {/* Session section */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Session</h2>
        <div className={styles.card}>
          <button
            className={styles.secondaryButton}
            onClick={() => logout()}
          >
            Sign Out
          </button>
        </div>
      </section>

      {/* Danger zone */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Account</h2>
        <div className={styles.dangerCard}>
          <p className={styles.dangerText}>
            Permanently delete your account and all associated data.
          </p>
          <button
            className={styles.dangerButton}
            onClick={() => setShowDeleteDialog(true)}
            disabled={isDeleting}
          >
            {isDeleting ? 'Deleting...' : 'Delete Account'}
          </button>
        </div>
      </section>

      {/* Attribution */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Attribution</h2>
        <div className={styles.card}>
          <ul className={styles.attributionList}>
            <li>Planning data provided by PlanIt (planit.org.uk)</li>
            <li>Contains public sector information licensed under the Open Government Licence. Crown Copyright.</li>
            <li>Contains Ordnance Survey data &copy; Crown Copyright and database right.</li>
            <li>Map data &copy; OpenStreetMap contributors.</li>
          </ul>
        </div>
      </section>

      {/* Legal links */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Legal</h2>
        <div className={styles.card}>
          <nav className={styles.legalLinks}>
            <Link to="/legal/privacy" className={styles.legalLink}>Privacy Policy</Link>
            <Link to="/legal/terms" className={styles.legalLink}>Terms of Service</Link>
          </nav>
        </div>
      </section>

      {error && <p className={styles.error}>{error}</p>}

      <ConfirmDialog
        open={showDeleteDialog}
        title="Delete Account"
        message="Are you sure you want to delete your account? This action cannot be undone. All your data will be permanently removed."
        confirmLabel="Delete"
        onConfirm={async () => {
          setShowDeleteDialog(false);
          await deleteAccount();
        }}
        onCancel={() => setShowDeleteDialog(false)}
      />
    </div>
  );
}
