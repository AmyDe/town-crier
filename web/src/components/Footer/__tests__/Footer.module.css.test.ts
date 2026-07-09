import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const cssPath = resolve(__dirname, '../Footer.module.css');

describe('Footer.module.css (small-caps section labels)', () => {
  const css = readFileSync(cssPath, 'utf-8');

  it('renders section labels in small caps, letterspaced', () => {
    const block = css.match(/\.sectionLabel\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('font-variant: small-caps');
    expect(block).toMatch(/letter-spacing:\s*0\.\d+em/);
  });
});
