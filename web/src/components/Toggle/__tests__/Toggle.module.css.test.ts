import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const cssPath = resolve(__dirname, '../Toggle.module.css');

describe('Toggle.module.css (token consumption)', () => {
  const css = readFileSync(cssPath, 'utf-8');

  it('uses a radius token instead of the magic 12px value', () => {
    const block = css.match(/\.toggle\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).not.toContain('border-radius: 12px');
    expect(block).toContain('border-radius: var(--tc-radius-lg)');
  });

  it('derives the 2px track padding from a spacing token instead of a magic value', () => {
    const block = css.match(/\.toggle\s*\{[^}]*\}/s)?.[0] ?? '';
    expect(block).not.toContain('padding: 2px');
    expect(block).toContain('padding: calc(var(--tc-space-xs) / 2)');
  });
});
