import { describe, it, expect } from 'vitest';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

const indexHtmlPath = resolve(__dirname, '../../index.html');
const globalCssPath = resolve(__dirname, '../styles/global.css');
const fontsDir = resolve(__dirname, '../../public/fonts');

function read(path: string): string {
  return readFileSync(path, 'utf-8');
}

describe('Self-hosted Inter font (no third-party transfer)', () => {
  describe('index.html', () => {
    const html = read(indexHtmlPath);

    it('does not preconnect to fonts.googleapis.com', () => {
      expect(html).not.toContain('fonts.googleapis.com');
    });

    it('does not preconnect to fonts.gstatic.com', () => {
      expect(html).not.toContain('fonts.gstatic.com');
    });

    it('does not reference any Google Fonts URL', () => {
      // Guard against future partial reverts (e.g. only removing preconnect).
      expect(html).not.toMatch(/https?:\/\/fonts\.(googleapis|gstatic)\.com/);
    });
  });

  describe('global.css', () => {
    const css = read(globalCssPath);

    it('declares an @font-face rule for Inter', () => {
      expect(css).toMatch(/@font-face\s*\{[^}]*font-family:\s*['"]?Inter['"]?/);
    });

    it('references local /fonts/ paths only, no Google domains', () => {
      expect(css).not.toContain('fonts.googleapis.com');
      expect(css).not.toContain('fonts.gstatic.com');
      // At least one @font-face src pointing at the local public/fonts path.
      expect(css).toMatch(/src:\s*url\(['"]?\/fonts\/[^)'"\s]+\.woff2/);
    });

    it('uses font-display: swap to avoid invisible text on slow networks', () => {
      expect(css).toContain('font-display: swap');
    });

    it('declares the Latin subset @font-face with the correct unicode-range', () => {
      // Matches the Google Fonts Latin subset — keeps the installed file small
      // while still covering UK English, currency symbols, and common punctuation.
      expect(css).toContain('U+0000-00FF');
    });

    it('declares the Latin-Extended subset @font-face with the correct unicode-range', () => {
      // Needed for European names (Brontë, café, naïve, etc.).
      expect(css).toContain('U+0100-02BA');
    });
  });

  describe('public/fonts directory', () => {
    it('contains the Inter Latin subset woff2 file', () => {
      expect(existsSync(resolve(fontsDir, 'inter-latin.woff2'))).toBe(true);
    });

    it('contains the Inter Latin-Extended subset woff2 file', () => {
      expect(existsSync(resolve(fontsDir, 'inter-latin-ext.woff2'))).toBe(true);
    });
  });
});
