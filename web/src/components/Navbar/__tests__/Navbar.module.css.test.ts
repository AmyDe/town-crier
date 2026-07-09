import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const cssPath = resolve(__dirname, '../Navbar.module.css');

describe('Navbar.module.css (masthead)', () => {
  const css = readFileSync(cssPath, 'utf-8');

  it('renders the wordmark in small caps with letter-spacing', () => {
    const block = css.match(/\.logo\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('font-variant: small-caps');
    expect(block).toMatch(/letter-spacing:\s*0\.\d+em/);
  });

  it('defines a 2.5px heavy rule under the masthead', () => {
    const block = css.match(/\.ruleHeavy\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('height: 2.5px');
  });

  it('defines a 1px hairline rule under the heavy rule', () => {
    const block = css.match(/\.ruleHairline\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('height: 1px');
  });
});
