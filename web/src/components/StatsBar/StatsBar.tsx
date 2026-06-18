import styles from './StatsBar.module.css';

interface Stat {
  value: string;
  label: string;
}

const STATS: Stat[] = [
  { value: 'UK-wide', label: 'Coverage' },
  { value: 'Free', label: 'To Get Started' },
  { value: 'Real-time', label: 'Push Alerts' },
];

export function StatsBar() {
  return (
    <section className={styles.container}>
      <ul className={styles.row}>
        {STATS.map((stat) => (
          <li key={stat.label} className={styles.item}>
            <span className={styles.value}>{stat.value}</span>
            <span className={styles.label}>{stat.label}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
