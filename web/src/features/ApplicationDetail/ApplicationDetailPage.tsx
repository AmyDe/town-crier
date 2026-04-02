import { useParams } from 'react-router-dom';
import { asApplicationUid } from '../../domain/types';
import type { ApplicationRepository } from '../../domain/ports/application-repository';
import type { DesignationRepository } from '../../domain/ports/designation-repository';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
import { formatDate, statusClassName } from '../../utils/formatting';
import { useApplication } from './useApplication';
import { useDesignations } from './useDesignations';
import { useSavedApplication } from './useSavedApplication';
import styles from './ApplicationDetailPage.module.css';

interface Props {
  applicationRepository: ApplicationRepository;
  designationRepository: DesignationRepository;
  savedApplicationRepository: SavedApplicationRepository;
}

export function ApplicationDetailPage({
  applicationRepository,
  designationRepository,
  savedApplicationRepository,
}: Props) {
  const { '*': rawUid } = useParams();
  const uid = asApplicationUid(rawUid ?? '');

  const { application, isLoading, error } = useApplication(applicationRepository, uid);
  const { designations } = useDesignations(
    designationRepository,
    application?.latitude ?? null,
    application?.longitude ?? null,
  );
  const { isSaved, toggleSave } = useSavedApplication(savedApplicationRepository, uid);

  if (isLoading) {
    return <div className={styles.loading}>Loading application...</div>;
  }

  if (error) {
    return <div className={styles.error}>{error}</div>;
  }

  if (!application) {
    return null;
  }

  const hasDesignations =
    designations !== null &&
    (designations.isWithinConservationArea ||
      designations.isWithinListedBuildingCurtilage ||
      designations.isWithinArticle4Area);

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <div className={styles.topRow}>
          <h1 className={styles.reference}>{application.name}</h1>
          <span
            className={`${styles.badge ?? ''} ${statusClassName(application.appState, styles)}`}
            role="status"
          >
            {application.appState}
          </span>
        </div>
        <p className={styles.address}>{application.address}</p>
      </header>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Description</h2>
        <p className={styles.description}>{application.description}</p>
      </section>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Details</h2>
        <div className={styles.detailGrid}>
          <div>
            <p className={styles.detailLabel}>Application Type</p>
            <p className={styles.detailValue}>{application.appType}</p>
          </div>
          <div>
            <p className={styles.detailLabel}>Authority</p>
            <p className={styles.detailValue}>{application.areaName}</p>
          </div>
          {application.startDate !== null && (
            <div>
              <p className={styles.detailLabel}>Received</p>
              <p className={styles.detailValue}>{formatDate(application.startDate)}</p>
            </div>
          )}
          {application.consultedDate && (
            <div>
              <p className={styles.detailLabel}>Consultation</p>
              <p className={styles.detailValue}>
                {formatDate(application.consultedDate)}
              </p>
            </div>
          )}
          {application.decidedDate && (
            <div>
              <p className={styles.detailLabel}>Decided</p>
              <p className={styles.detailValue}>
                {formatDate(application.decidedDate)}
              </p>
            </div>
          )}
        </div>
      </section>

      {hasDesignations && (
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>Designations</h2>
          <ul className={styles.designationList}>
            {designations.isWithinConservationArea && (
              <li className={styles.designationItem}>
                Conservation Area: {designations.conservationAreaName}
              </li>
            )}
            {designations.isWithinListedBuildingCurtilage && (
              <li className={styles.designationItem}>
                Listed Building: {designations.listedBuildingGrade}
              </li>
            )}
            {designations.isWithinArticle4Area && (
              <li className={styles.designationItem}>
                Article 4 Direction
              </li>
            )}
          </ul>
        </section>
      )}

      <div className={styles.actions}>
        <button
          className={`${styles.saveButton} ${isSaved ? styles.saved : ''}`}
          onClick={toggleSave}
          type="button"
        >
          {isSaved ? 'Saved' : 'Save'}
        </button>

        {application.url && (
          <a
            className={styles.portalLink}
            href={application.url}
            target="_blank"
            rel="noopener noreferrer"
          >
            View on Council Portal
          </a>
        )}
      </div>
    </div>
  );
}
