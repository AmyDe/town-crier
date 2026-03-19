import styles from './Pricing.module.css';

interface Feature {
  label: string;
  free: string;
  personal: string;
  pro: string;
}

const FEATURES: Feature[] = [
  { label: 'Watch Zones', free: '1', personal: '5', pro: 'Unlimited' },
  { label: 'Radius', free: '500m', personal: '2km', pro: '5km' },
  { label: 'Notifications', free: 'Weekly digest', personal: 'Instant', pro: 'Instant' },
  { label: 'Search', free: 'Basic', personal: 'Advanced', pro: 'Advanced + Filters' },
  { label: 'Historical Data', free: '—', personal: '6 months', pro: '5 years' },
];

interface Tier {
  name: string;
  price: string;
  period?: string;
  recommended?: boolean;
  trialText?: string;
}

const TIERS: Tier[] = [
  { name: 'Free', price: '£0' },
  { name: 'Personal', price: '£1.99', period: '/mo', recommended: true, trialText: '14-day free trial' },
  { name: 'Pro', price: '£5.99', period: '/mo' },
];

export function Pricing() {
  return (
    <section id="pricing" className={styles.container}>
      <h2 className={styles.heading}>Pricing</h2>
      <div className={styles.grid}>
        {TIERS.map((tier) => (
          <article
            key={tier.name}
            className={`${styles.card} ${tier.recommended ? styles.recommended : ''}`}
          >
            {tier.recommended && (
              <span className={styles.badge}>Recommended</span>
            )}
            <h3 className={styles.tierName}>{tier.name}</h3>
            <p className={styles.price}>
              <span className={styles.priceAmount}>{tier.price}</span>
              {tier.period && <span className={styles.pricePeriod}>{tier.period}</span>}
            </p>
            {tier.trialText && (
              <p className={styles.trialText}>{tier.trialText}</p>
            )}
            <ul className={styles.featureList}>
              {FEATURES.map((feature) => {
                const value =
                  tier.name === 'Free'
                    ? feature.free
                    : tier.name === 'Personal'
                      ? feature.personal
                      : feature.pro;
                return (
                  <li key={feature.label} className={styles.featureRow}>
                    <span className={styles.featureLabel}>{feature.label}</span>
                    <span className={styles.featureValue}>{value}</span>
                  </li>
                );
              })}
            </ul>
          </article>
        ))}
      </div>
    </section>
  );
}
