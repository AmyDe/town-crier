import { useParams } from 'react-router-dom';
import styles from './LegalPage.module.css';

const LEGAL_TITLES: Record<string, string> = {
  privacy: 'Privacy Policy',
  terms: 'Terms of Service',
};

export function LegalPage() {
  const { type } = useParams<{ type: string }>();
  const title = (type && LEGAL_TITLES[type]) ?? 'Legal';

  return (
    <div className={styles.container}>
      <h1 className={styles.title}>{title}</h1>
      <p className={styles.description}>This page is coming soon.</p>
    </div>
  );
}
