export interface Feature {
  label: string;
  free: string;
  personal: string;
  pro: string;
}

export const FEATURES: Feature[] = [
  { label: 'Watch Zones', free: '1', personal: '3', pro: 'Unlimited' },
  { label: 'Radius', free: '2km', personal: '5km', pro: '10km' },
  { label: 'Custom Shapes', free: 'Circle only', personal: 'Yes', pro: 'Yes' },
  { label: 'Notifications', free: 'Weekly digest', personal: 'Instant', pro: 'Instant' },
  { label: 'Webhooks (JSON or Slack)', free: 'No', personal: 'No', pro: 'Yes' },
];

export interface AltPrice {
  amount: string;
  period: string;
}

export interface Tier {
  name: string;
  price: string;
  period?: string;
  recommended?: boolean;
  trialText?: string;
  altPrices?: AltPrice[];
  note?: string;
}

export const TIERS: Tier[] = [
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
