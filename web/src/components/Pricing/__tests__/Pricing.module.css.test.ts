import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const cssPath = resolve(__dirname, '../Pricing.module.css');

describe('Pricing.module.css (filed-notice tiers + rationed amber upsell)', () => {
  const css = readFileSync(cssPath, 'utf-8');

  it('gives every tier card a top rule (filed-notice pattern)', () => {
    const block = css.match(/\.card\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('border-top: 2px solid var(--tc-text-primary)');
  });

  it('gives the recommended tier a 1.5px amber border, not a filled background', () => {
    const block = css.match(/\.recommended\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('border: 1.5px solid var(--tc-amber)');
    expect(block).not.toContain('background: var(--tc-amber)');
  });

  it('renders the eyebrow as small-caps letterspaced text, not a filled pill', () => {
    const block = css.match(/\.eyebrow\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('text-transform: uppercase');
    expect(block).toMatch(/letter-spacing:\s*0\.\d+em/);
    expect(block).not.toContain('background: var(--tc-amber-muted)');
  });

  it('is the only filled-amber container: the CTA button', () => {
    const block = css.match(/\.cta\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('background: var(--tc-amber)');
  });
});
