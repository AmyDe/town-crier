import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const cssPath = resolve(__dirname, '../ConfirmDialog.module.css');

describe('ConfirmDialog.module.css (token consumption)', () => {
  const css = readFileSync(cssPath, 'utf-8');

  it('does not hard-code #ffffff — uses the text-on-accent token', () => {
    expect(css).not.toContain('#ffffff');
    const block = css.match(/\.confirmButton\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('color: var(--tc-text-on-accent)');
  });
});
