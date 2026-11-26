// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { FAQ_ITEMS } from '../../components/Faq/faqItems';
import { TIERS } from '../../components/Pricing/plans';
import { buildLandingJsonLd } from '../../features/LandingPage/structuredData';
import { renderLanding } from '../landing';

function asText(html: string): string {
  return html
    .replaceAll('&#x27;', "'")
    .replaceAll('&quot;', '"')
    .replaceAll('&amp;', '&');
}

describe('renderLanding', () => {
  it('renders the landing page without a DOM', () => {
    expect(typeof window).toBe('undefined');

    const text = asText(renderLanding().html);

    expect(text).toMatch(
      /<h1[^>]*>Stay informed about what's being built in your neighbourhood<\/h1>/,
    );
    expect(text).toContain('Pricing');
    for (const tier of TIERS) {
      expect(text).toContain(tier.price);
      for (const alt of tier.altPrices ?? []) {
        expect(text).toContain(alt.amount);
      }
    }
    for (const item of FAQ_ITEMS) {
      expect(text).toContain(item.question);
    }
  });

  it('returns the structured data as JSON', () => {
    expect(JSON.parse(renderLanding().jsonLd)).toEqual(buildLandingJsonLd());
  });
});
