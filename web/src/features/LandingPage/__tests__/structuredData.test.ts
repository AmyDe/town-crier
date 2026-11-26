import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { FAQ_ITEMS } from '../../../components/Faq/faqItems';
import { TIERS } from '../../../components/Pricing/plans';
import { APP_DOWNLOAD_URL } from '../../../config/links';
import { buildLandingJsonLd } from '../structuredData';

function objectOfType(type: string) {
  const match = buildLandingJsonLd().find((entry) => entry['@type'] === type);
  if (!match) {
    throw new Error(`No ${type} object`);
  }
  return match;
}

describe('buildLandingJsonLd', () => {
  it('returns the four schema.org objects', () => {
    const objects = buildLandingJsonLd();

    expect(objects.map((entry) => entry['@type'])).toEqual([
      'Organization',
      'MobileApplication',
      'WebApplication',
      'FAQPage',
    ]);
    for (const entry of objects) {
      expect(entry['@context']).toBe('https://schema.org');
    }
  });

  it('describes the organisation and links it to the App Store', () => {
    expect(objectOfType('Organization')).toMatchObject({
      name: 'Town Crier',
      url: 'https://towncrierapp.uk/',
      logo: 'https://towncrierapp.uk/android-chrome-512x512.png',
      sameAs: [APP_DOWNLOAD_URL],
    });
  });

  it('describes the iPhone app with the llms.txt summary', () => {
    const llms = readFileSync(
      resolve(__dirname, '../../../../public/llms.txt'),
      'utf-8',
    );
    const app = objectOfType('MobileApplication');

    expect(app).toMatchObject({
      name: 'Town Crier: Planning Alerts',
      operatingSystem: 'iOS 17.0 or later',
      applicationCategory: 'NewsApplication',
      url: APP_DOWNLOAD_URL,
    });
    expect(llms).toContain(`> ${String(app['description'])}\n`);
  });

  it('offers every price in TIERS in GBP', () => {
    const tierPrices = TIERS.flatMap((tier) => [
      tier.price,
      ...(tier.altPrices ?? []).map((alt) => alt.amount),
    ]).map((price) => price.replace('£', ''));
    const offers = objectOfType('MobileApplication')['offers'];

    expect(tierPrices).toHaveLength(5);
    expect(offers).toEqual(
      ['Free', 'Personal, monthly', 'Pro, monthly', 'Pro, yearly', 'Pro, lifetime'].map(
        (name, index) => ({
          '@type': 'Offer',
          name,
          price: tierPrices[index],
          priceCurrency: 'GBP',
        }),
      ),
    );
  });

  it('describes the web app', () => {
    expect(objectOfType('WebApplication')).toMatchObject({
      name: 'Town Crier',
      url: 'https://towncrierapp.uk/',
      applicationCategory: 'NewsApplication',
      operatingSystem: 'Any web browser',
    });
  });

  it('has one question per FAQ item', () => {
    expect(objectOfType('FAQPage')['mainEntity']).toEqual(
      FAQ_ITEMS.map((item) => ({
        '@type': 'Question',
        name: item.question,
        acceptedAnswer: { '@type': 'Answer', text: item.answer },
      })),
    );
  });

  it('carries no aggregate rating', () => {
    expect(JSON.stringify(buildLandingJsonLd())).not.toContain('aggregateRating');
  });
});
