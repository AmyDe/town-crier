import { appStoreUrl } from '../../config/links';
import styles from './Pricing.module.css';

interface Feature {
  label: string;
  free: string;
  personal: string;
  pro: string;
}

const FEATURES: Feature[] = [
  { label: 'Watch Zones', free: '1', personal: '3', pro: 'Unlimited' },
  { label: 'Radius', free: '2km', personal: '5km', pro: '10km' },
  { label: 'Custom Shapes', free: 'Circle only', personal: 'Yes', pro: 'Yes' },
  { label: 'Notifications', free: 'Weekly digest', personal: 'Instant', pro: 'Instant' },
];

interface AltPrice {
  amount: string;
  period: string;
}

interface Tier {
  name: string;
  price: string;
  period?: string;
  recommended?: boolean;
  trialText?: string;
  altPrices?: AltPrice[];
  note?: string;
}

const TIERS: Tier[] = [
  { name: 'Free', price: '£0' },
  { name: 'Personal', price: '£1.99', period: '/mo', recommended: true, trialText: '7-day free trial' },
  {
    name: 'Pro',
    price: '£4.99',
    period: '/mo',
    altPrices: [
      { amount: '£29.99', period: '/yr' },
      { amount: '£69.99', period: 'once, for life' },
    ],
    note: 'Yearly and lifetime plans are in the iOS app.',
  },
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
              <span className={styles.eyebrow}>Recommended</span>
            )}
            <h3 className={styles.tierName}>{tier.name}</h3>
            <p className={styles.price}>
              <span className={styles.priceAmount}>{tier.price}</span>
              {tier.period && <span className={styles.pricePeriod}>{tier.period}</span>}
            </p>
            {tier.trialText && (
              <p className={styles.trialText}>{tier.trialText}</p>
            )}
            {tier.altPrices?.map((alt) => (
              <p key={alt.amount} className={styles.altPrice}>
                <span className={styles.altPriceAmount}>{alt.amount}</span>
                <span className={styles.pricePeriod}>{alt.period}</span>
              </p>
            ))}
            {tier.note && <p className={styles.trialText}>{tier.note}</p>}
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
            {tier.recommended && (
              <a
                className={styles.cta}
                href={appStoreUrl('web-pricing')}
                target="_blank"
                rel="noopener noreferrer"
              >
                Start your trial
              </a>
            )}
          </article>
        ))}
      </div>
    </section>
  );
}
