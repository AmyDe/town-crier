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

  it('explicitly welcomes the major AI/LLM crawlers', () => {
    const content = robots();
    const aiCrawlers = [
      'GPTBot',
      'OAI-SearchBot',
      'ChatGPT-User',
      'Google-Extended',
      'ClaudeBot',
      'Claude-User',
      'anthropic-ai',
      'PerplexityBot',
      'Perplexity-User',
      'CCBot',
      'Applebot-Extended',
    ];
    for (const crawler of aiCrawlers) {
      expect(content).toContain(`User-agent: ${crawler}`);
    }
  });
});

describe('llms.txt', () => {
  function llms(): string {
    return readFileSync(resolve(__dirname, '../../public/llms.txt'), 'utf-8');
  }

  it('is an llms.txt with the site name as the H1 title', () => {
    expect(llms()).toContain('# Town Crier');
  });

  it('points LLMs at the sitemap of all planning pages', () => {
    expect(llms()).toContain('https://towncrierapp.uk/sitemap.xml');
  });
});
