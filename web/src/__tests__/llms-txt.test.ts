import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { TIERS } from '../components/Pricing/plans';
import { APP_DOWNLOAD_URL } from '../config/links';

const llms = readFileSync(resolve(__dirname, '../../public/llms.txt'), 'utf-8');

describe('llms.txt product copy', () => {
  it('names every plan with each of its prices', () => {
    for (const tier of TIERS) {
      expect(llms).toContain(`- ${tier.name}, `);
      expect(llms).toContain(tier.price);
      for (const alt of tier.altPrices ?? []) {
        expect(llms).toContain(alt.amount);
      }
    }
  });

  it('links to the App Store listing and the planning pages', () => {
    expect(llms).toContain(APP_DOWNLOAD_URL);
    expect(llms).toContain('https://towncrierapp.uk/planning');
  });

  it('uses no em or en dashes', () => {
    expect(llms).not.toMatch(/[–—]/);
  });
});
