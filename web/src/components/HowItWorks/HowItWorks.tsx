import styles from './HowItWorks.module.css';

interface Step {
  icon: string;
  title: string;
  description: string;
}

const STEPS: Step[] = [
  {
    icon: '📍',
    title: 'Enter your postcode',
    description:
      'Tell us where you live or work. We use your postcode to find the local authority that handles planning in your area.',
  },
  {
    icon: '🔭',
    title: 'Create a watch zone',
    description:
      'Draw a circle around the area you care about — your street, neighbourhood, or an entire ward. You choose the radius.',
  },
  {
    icon: '🔔',
    title: 'Get notified',
    description:
      'Receive a push notification whenever a new planning application is submitted within your watch zone. No more missed deadlines.',
  },
];

export function HowItWorks() {
  return (
    <section id="how-it-works" className={styles.section}>
      <h2 className={styles.heading}>How It Works</h2>
      <ol className={styles.grid}>
        {STEPS.map((step) => (
          <li key={step.title} className={styles.card}>
            <span className={styles.icon} aria-hidden="true">
              {step.icon}
            </span>
            <h3 className={styles.stepTitle}>{step.title}</h3>
            <p className={styles.stepDescription}>{step.description}</p>
          </li>
        ))}
      </ol>
    </section>
  );
}
