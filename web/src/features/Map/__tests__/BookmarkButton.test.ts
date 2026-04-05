import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

const cssPath = resolve(__dirname, '../BookmarkButton.module.css');

function readCss(): string {
  return readFileSync(cssPath, 'utf-8');
}

describe('BookmarkButton styles', () => {
  it('defines a :focus-visible outline using --tc-amber for keyboard accessibility', () => {
    const css = readCss();

    expect(css).toContain(':focus-visible');
    expect(css).toContain('outline');
    expect(css).toContain('var(--tc-amber)');
  });
});
