import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

describe('robots.txt SEO additions', () => {
  function robots(): string {
    return readFileSync(
      resolve(__dirname, '../../public/robots.txt'),
      'utf-8',
    );
  }

  it('still allows all crawlers', () => {
    const content = robots();
    expect(content).toContain('User-agent: *');
    expect(content).toContain('Allow: /');
  });

  it('references the absolute sitemap URL', () => {
    expect(robots()).toContain(
      'Sitemap: https://towncrierapp.uk/sitemap.xml',
    );
  });
});
