import styles from './PlaceholderPage.module.css';

interface Props {
  title: string;
}

export function PlaceholderPage({ title }: Props) {
  return (
    <div className={styles.container}>
      <h1 className={styles.title}>{title}</h1>
      <p className={styles.description}>This page is coming soon.</p>
    </div>
  );
}
