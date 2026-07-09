import { describe, it, expect } from 'vitest';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

const indexHtmlPath = resolve(__dirname, '../../index.html');
const globalCssPath = resolve(__dirname, '../styles/global.css');
const fontsDir = resolve(__dirname, '../../public/fonts');

function read(path: string): string {
  return readFileSync(path, 'utf-8');
}

describe('Self-hosted Fraunces font (Public Notice display face, no third-party transfer)', () => {
  describe('global.css', () => {
    const css = read(globalCssPath);

    it('declares an @font-face rule for Fraunces at weight 400', () => {
      expect(css).toMatch(
        /@font-face\s*\{[^}]*font-family:\s*['"]?Fraunces['"]?[^}]*font-weight:\s*400[^}]*\}/s,
      );
    });

    it('declares an @font-face rule for Fraunces at weight 600', () => {
      expect(css).toMatch(
        /@font-face\s*\{[^}]*font-family:\s*['"]?Fraunces['"]?[^}]*font-weight:\s*600[^}]*\}/s,
      );
    });

    it('does not declare any other Fraunces weight (latin subset, 400+600 only)', () => {
      const fraunces400Or600 = /font-family:\s*['"]?Fraunces['"]?[\s\S]*?font-weight:\s*(400|600);/g;
      const allFraunces = css.match(/@font-face\s*\{[^}]*font-family:\s*['"]?Fraunces['"]?[^}]*\}/gs) ?? [];
      expect(allFraunces).toHaveLength(2);
      expect(css.match(fraunces400Or600)).not.toBeNull();
    });

    it('uses font-display: swap for Fraunces', () => {
      const fraunces = css.match(/@font-face\s*\{[^}]*font-family:\s*['"]?Fraunces['"]?[^}]*\}/gs) ?? [];
      for (const block of fraunces) {
        expect(block).toContain('font-display: swap');
      }
    });

    it('references local /fonts/ paths only, no Google domains', () => {
      const fraunces = css.match(/@font-face\s*\{[^}]*font-family:\s*['"]?Fraunces['"]?[^}]*\}/gs) ?? [];
      for (const block of fraunces) {
        expect(block).toMatch(/src:\s*url\(['"]?\/fonts\/fraunces-latin-(400|600)-normal\.woff2/);
        expect(block).not.toContain('fonts.googleapis.com');
        expect(block).not.toContain('fonts.gstatic.com');
      }
    });
  });

  describe('index.html', () => {
    const html = read(indexHtmlPath);

    it('preloads the Fraunces 600 weight only (display roles are semibold)', () => {
      expect(html).toMatch(
        /<link rel="preload" href="\/fonts\/fraunces-latin-600-normal\.woff2" as="font" type="font\/woff2" crossorigin/,
      );
      expect(html).not.toContain('fraunces-latin-400-normal.woff2');
    });

    it('does not reference any Google Fonts URL', () => {
      expect(html).not.toMatch(/https?:\/\/fonts\.(googleapis|gstatic)\.com/);
    });
  });

  describe('public/fonts directory', () => {
    it('contains the Fraunces 400 latin subset woff2 file', () => {
      expect(existsSync(resolve(fontsDir, 'fraunces-latin-400-normal.woff2'))).toBe(true);
    });

    it('contains the Fraunces 600 latin subset woff2 file', () => {
      expect(existsSync(resolve(fontsDir, 'fraunces-latin-600-normal.woff2'))).toBe(true);
    });
  });
});
