import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const cssPath = resolve(__dirname, '../Hero.module.css');

describe('Hero.module.css (Public Notice display type)', () => {
  const css = readFileSync(cssPath, 'utf-8');

  it('sets the headline in the Fraunces display face', () => {
    const block = css.match(/\.headline\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('font-family: var(--tc-font-display)');
  });
});
