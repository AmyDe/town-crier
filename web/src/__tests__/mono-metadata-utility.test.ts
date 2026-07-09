import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const globalCssPath = resolve(__dirname, '../styles/global.css');

describe('Mono metadata utility (global.css)', () => {
  const css = readFileSync(globalCssPath, 'utf-8');

  it('declares a .tc-mono-meta utility class', () => {
    expect(css).toMatch(/\.tc-mono-meta\s*\{/);
  });

  it('sets the mono font family for planning refs/dates/distances', () => {
    const block = css.match(/\.tc-mono-meta\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('font-family: var(--tc-font-mono)');
  });

  it('uses tabular figures so numbers align in a column', () => {
    const block = css.match(/\.tc-mono-meta\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).toContain('font-variant-numeric: tabular-nums');
  });
});
