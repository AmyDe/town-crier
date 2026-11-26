import { FAQ_ITEMS } from '../../components/Faq/faqItems';
import { TIERS } from '../../components/Pricing/plans';
import { APP_DOWNLOAD_URL } from '../../config/links';

const SITE_URL = 'https://towncrierapp.uk/';

const APP_DESCRIPTION =
  'Town Crier tells people in the UK about planning applications near them. Pick an area on a map and you get an alert when an application is submitted or decided there. It covers every local planning authority in England, Scotland, Wales and Northern Ireland.';

const BILLING_LABELS: Record<string, string> = {
  '/mo': 'monthly',
  '/yr': 'yearly',
  'once, for life': 'lifetime',
};

interface JsonLdObject {
  '@context': 'https://schema.org';
  '@type': string;
  [property: string]: unknown;
}

function offer(tierName: string, amount: string, period?: string) {
  const billing = period === undefined ? undefined : (BILLING_LABELS[period] ?? period);
  return {
    '@type': 'Offer',
    name: billing === undefined ? tierName : `${tierName}, ${billing}`,
    price: amount.replace('£', ''),
    priceCurrency: 'GBP',
  };
}

/** schema.org objects that describe Town Crier on the landing page. */
export function buildLandingJsonLd(): JsonLdObject[] {
  const offers = TIERS.flatMap((tier) => [
    offer(tier.name, tier.price, tier.period),
    ...(tier.altPrices ?? []).map((alt) => offer(tier.name, alt.amount, alt.period)),
  ]);

  return [
    {
      '@context': 'https://schema.org',
      '@type': 'Organization',
      name: 'Town Crier',
      url: SITE_URL,
      logo: `${SITE_URL}android-chrome-512x512.png`,
      sameAs: [APP_DOWNLOAD_URL],
    },
    {
      '@context': 'https://schema.org',
      '@type': 'MobileApplication',
      name: 'Town Crier: Planning Alerts',
      operatingSystem: 'iOS 17.0 or later',
      applicationCategory: 'NewsApplication',
      url: APP_DOWNLOAD_URL,
      description: APP_DESCRIPTION,
      offers,
    },
    {
      '@context': 'https://schema.org',
      '@type': 'WebApplication',
      name: 'Town Crier',
      url: SITE_URL,
      applicationCategory: 'NewsApplication',
      operatingSystem: 'Any web browser',
    },
    {
      '@context': 'https://schema.org',
      '@type': 'FAQPage',
      mainEntity: FAQ_ITEMS.map((item) => ({
        '@type': 'Question',
        name: item.question,
        acceptedAnswer: { '@type': 'Answer', text: item.answer },
      })),
    },
  ];
}
