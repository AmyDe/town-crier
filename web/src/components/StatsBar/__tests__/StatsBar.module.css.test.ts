import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const cssPath = resolve(__dirname, '../StatsBar.module.css');

describe('StatsBar.module.css (mono tabular numbers)', () => {
  const css = readFileSync(cssPath, 'utf-8');

  it('sets the stat value in the mono face with tabular figures', () => {
    const block = css.match(/\.value\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('font-family: var(--tc-font-mono)');
    expect(block).toContain('font-variant-numeric: tabular-nums');
  });
});
