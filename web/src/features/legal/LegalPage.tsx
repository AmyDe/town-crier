import { useParams } from 'react-router-dom';
import type { LegalDocumentPort } from '../../domain/ports/legal-document-port';
import { useLegalDocument } from './useLegalDocument';
import styles from './LegalPage.module.css';

interface Props {
  port: LegalDocumentPort;
}

function formatDate(dateString: string): string {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

export function LegalPage({ port }: Props) {
  const { type } = useParams<{ type: string }>();
  const documentType = type ?? 'privacy';
  const { document, isLoading, error } = useLegalDocument(port, documentType);

  if (isLoading) {
    return (
      <div className={styles.container}>
        <p className={styles.loading}>Loading...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.container}>
        <p className={styles.error}>{error}</p>
      </div>
    );
  }

  if (!document) {
    return (
      <div className={styles.container}>
        <h1 className={styles.title}>Legal</h1>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.title}>{document.title}</h1>
      <p className={styles.lastUpdated}>Last updated: {formatDate(document.lastUpdated)}</p>
      <div className={styles.sections}>
        {document.sections.map((section) => (
          <section key={section.heading} className={styles.section}>
            <h2 className={styles.sectionHeading}>{section.heading}</h2>
            <p className={styles.sectionBody}>{section.body}</p>
          </section>
        ))}
      </div>
    </div>
  );
}
