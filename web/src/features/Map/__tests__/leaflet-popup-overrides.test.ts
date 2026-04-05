import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

/**
 * Leaflet ships popup styles with a hardcoded white background.
 * In dark mode the app's text tokens resolve to light colours,
 * making popup text invisible. These tests guarantee that override
 * styles exist and use the correct design tokens so the bug cannot
 * silently regress.
 */
describe('Leaflet popup dark-mode overrides', () => {
  const cssPath = resolve(__dirname, '..', 'leaflet-overrides.css');
  let css: string;

  try {
    css = readFileSync(cssPath, 'utf-8');
  } catch {
    css = '';
  }

  it('override stylesheet exists', () => {
    expect(css.length).toBeGreaterThan(0);
  });

  it('overrides .leaflet-popup-content-wrapper background with --tc-surface', () => {
    expect(css).toContain('.leaflet-popup-content-wrapper');
    expect(css).toMatch(/\.leaflet-popup-content-wrapper[^}]*var\(--tc-surface\)/s);
  });

  it('overrides .leaflet-popup-tip background with --tc-surface', () => {
    expect(css).toContain('.leaflet-popup-tip');
    expect(css).toMatch(/\.leaflet-popup-tip[^}]*var\(--tc-surface\)/s);
  });

  it('sets popup text color to --tc-text-primary', () => {
    expect(css).toMatch(/\.leaflet-popup-content-wrapper[^}]*var\(--tc-text-primary\)/s);
  });

  it('sets popup link color to --tc-amber', () => {
    expect(css).toMatch(/\.leaflet-popup-content[^}]*var\(--tc-amber\)/s);
  });
});
