import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const cssPath = resolve(__dirname, '../Faq.module.css');

describe('Faq.module.css (ledger rows)', () => {
  const css = readFileSync(cssPath, 'utf-8');

  it('draws a heavy rule at the top of the section', () => {
    const block = css.match(/\.section\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('border-top: 2.5px solid var(--tc-text-primary)');
  });

  it('separates ledger rows with a hairline rule instead of boxed cards', () => {
    const block = css.match(/\.details\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('border-bottom: 1px solid var(--tc-border)');
    expect(block).not.toContain('border-radius');
  });

  it('does not use amber for the expand/collapse marker (rationed)', () => {
    expect(css).not.toContain('var(--tc-amber)');
  });
});
