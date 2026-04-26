import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, it, expect } from 'vitest';

const tokensPath = resolve(__dirname, '../styles/tokens.css');
const globalPath = resolve(__dirname, '../styles/global.css');

function readCss(path: string): string {
  return readFileSync(path, 'utf-8');
}

describe('Design tokens (tokens.css)', () => {
  const css = readCss(tokensPath);

  it('defines dark theme colour tokens on :root (default)', () => {
    expect(css).toContain('--tc-background: #1A1A1E');
    expect(css).toContain('--tc-surface: #242428');
    expect(css).toContain('--tc-surface-elevated: #2E2E33');
    expect(css).toContain('--tc-amber: #E9A620');
    expect(css).toContain('--tc-text-primary: #F1EFE9');
    expect(css).toContain('--tc-text-secondary: #9B9590');
    expect(css).toContain('--tc-text-tertiary: #5C5852');
    expect(css).toContain('--tc-text-on-accent: #1C1917');
  });

  it('defines light theme tokens under [data-theme="light"]', () => {
    expect(css).toContain('[data-theme="light"]');
    expect(css).toContain('--tc-background: #FAF8F5');
    expect(css).toContain('--tc-surface: #FFFFFF');
    expect(css).toContain('--tc-amber: #D4910A');
    expect(css).toContain('--tc-text-primary: #1C1917');
    expect(css).toContain('--tc-text-secondary: #6B6560');
  });

  it('defines OLED dark tokens under [data-theme="oled-dark"]', () => {
    expect(css).toContain('[data-theme="oled-dark"]');
    expect(css).toContain('--tc-background: #000000');
    expect(css).toContain('--tc-surface: #0A0A0A');
    expect(css).toContain('--tc-surface-elevated: #161616');
  });

  it('defines status tokens for all PlanIt application states', () => {
    expect(css).toContain('--tc-status-permitted');
    expect(css).toContain('--tc-status-conditions');
    expect(css).toContain('--tc-status-rejected');
    expect(css).toContain('--tc-status-pending');
    expect(css).toContain('--tc-status-withdrawn');
    expect(css).toContain('--tc-status-appealed');
  });

  it('does not define legacy status tokens (Approved/Refused)', () => {
    // PlanIt uses Permitted/Conditions/Rejected; legacy tokens were renamed.
    expect(css).not.toContain('--tc-status-approved');
    expect(css).not.toContain('--tc-status-refused');
  });

  it('defines spacing scale tokens', () => {
    expect(css).toContain('--tc-space-xs: 4px');
    expect(css).toContain('--tc-space-sm: 8px');
    expect(css).toContain('--tc-space-md: 16px');
    expect(css).toContain('--tc-space-lg: 24px');
    expect(css).toContain('--tc-space-xl: 32px');
    expect(css).toContain('--tc-space-xxl: 48px');
  });

  it('defines corner radius tokens', () => {
    expect(css).toContain('--tc-radius-sm: 8px');
    expect(css).toContain('--tc-radius-md: 12px');
    expect(css).toContain('--tc-radius-lg: 16px');
    expect(css).toContain('--tc-radius-full: 9999px');
  });

  it('defines typography tokens', () => {
    expect(css).toContain('--tc-font-family');
    expect(css).toContain('--tc-text-display-large');
    expect(css).toContain('--tc-text-display-small');
    expect(css).toContain('--tc-text-headline');
    expect(css).toContain('--tc-text-body');
    expect(css).toContain('--tc-text-caption');
    expect(css).toContain('--tc-weight-regular');
    expect(css).toContain('--tc-weight-semibold');
    expect(css).toContain('--tc-weight-bold');
  });

  it('defines shadow tokens', () => {
    expect(css).toContain('--tc-shadow-card');
    expect(css).toContain('--tc-shadow-elevated');
  });

  it('includes prefers-color-scheme media query for first-visit detection', () => {
    expect(css).toContain('@media (prefers-color-scheme: light)');
    expect(css).toContain(':root:not([data-theme])');
  });
});

describe('Global styles (global.css)', () => {
  const css = readCss(globalPath);

  it('applies box-sizing reset', () => {
    expect(css).toContain('box-sizing: border-box');
  });

  it('sets body font using design tokens', () => {
    expect(css).toContain('font-family: var(--tc-font-family)');
    expect(css).toContain('color: var(--tc-text-primary)');
    expect(css).toContain('background-color: var(--tc-background)');
  });

  it('styles headings using design tokens', () => {
    expect(css).toContain('font-size: var(--tc-text-display-large)');
    expect(css).toContain('font-size: var(--tc-text-display-small)');
    expect(css).toContain('font-size: var(--tc-text-headline)');
  });

  it('styles links with amber accent', () => {
    expect(css).toContain('color: var(--tc-amber)');
    expect(css).toContain('color: var(--tc-amber-hover)');
  });

  it('includes focus-visible outlines for accessibility', () => {
    expect(css).toContain('focus-visible');
    expect(css).toContain('var(--tc-border-focused)');
  });

  it('includes reduced motion media query', () => {
    expect(css).toContain('@media (prefers-reduced-motion: reduce)');
  });
});
