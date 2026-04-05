import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth0 } from '@auth0/auth0-react';
import { useTheme } from '../../hooks/useTheme';
import { useUserProfile } from './useUserProfile';
import { ThemeToggle } from '../../components/ThemeToggle/ThemeToggle';
import { ConfirmDialog } from '../../components/ConfirmDialog/ConfirmDialog';
import type { SettingsRepository } from '../../domain/ports/settings-repository';
import styles from './SettingsPage.module.css';

interface Props {
  repository: SettingsRepository;
}

export function SettingsPage({ repository }: Props) {
  const { logout } = useAuth0();
  const { theme, toggleTheme } = useTheme();
  const {
    profile,
    isLoading,
    isExporting,
    isDeleting,
    error,
    exportData,
    deleteAccount,
  } = useUserProfile(repository, () => logout());

  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

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
