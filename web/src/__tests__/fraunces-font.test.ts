import { describe, it, expect } from 'vitest';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

const indexHtmlPath = resolve(__dirname, '../../index.html');
const globalCssPath = resolve(__dirname, '../styles/global.css');
const fontsDir = resolve(__dirname, '../../public/fonts');

function read(path: string): string {
  return readFileSync(path, 'utf-8');
}

// Fraunces (the Public Notice display-serif face) was dropped in favour of a
// single sans stack everywhere, owner-approved on tester feedback (GH #912,
// tc-b3nki.7, 2026-07-10). These tests guard against the self-hosted face,
// its @font-face declarations, and its woff2 assets ever creeping back in.
describe('Sans-only typography (Fraunces display face removed, tc-b3nki.7)', () => {
  describe('global.css', () => {
    const css = read(globalCssPath);

    it('declares no @font-face rule for Fraunces', () => {
      expect(css).not.toMatch(/font-family:\s*['"]?Fraunces['"]?/);
    });

    it('still self-hosts Inter with font-display: swap (unaffected by the display-face removal)', () => {
      const interBlocks = css.match(/@font-face\s*\{[^}]*font-family:\s*['"]?Inter['"]?[^}]*\}/gs) ?? [];
      expect(interBlocks.length).toBeGreaterThan(0);
      for (const block of interBlocks) {
        expect(block).toContain('font-display: swap');
      }
    });
  });

  describe('index.html', () => {
    const html = read(indexHtmlPath);

    it('does not preload any Fraunces font file', () => {
      expect(html).not.toMatch(/fraunces/i);
    });

    it('still preloads the self-hosted Inter latin subset', () => {
      expect(html).toMatch(
        /<link rel="preload" href="\/fonts\/inter-latin\.woff2" as="font" type="font\/woff2" crossorigin/,
      );
    });

    it('does not reference any Google Fonts URL', () => {
      expect(html).not.toMatch(/https?:\/\/fonts\.(googleapis|gstatic)\.com/);
    });
  });

  describe('public/fonts directory', () => {
    it('no longer contains the Fraunces 400 latin subset woff2 file', () => {
      expect(existsSync(resolve(fontsDir, 'fraunces-latin-400-normal.woff2'))).toBe(false);
    });

    it('no longer contains the Fraunces 600 latin subset woff2 file', () => {
      expect(existsSync(resolve(fontsDir, 'fraunces-latin-600-normal.woff2'))).toBe(false);
    });
  });
});
