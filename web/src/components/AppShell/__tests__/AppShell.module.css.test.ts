import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const cssPath = resolve(__dirname, '../AppShell.module.css');

describe('AppShell.module.css (mobile overflow regression, tc-q2h0s)', () => {
  const css = readFileSync(cssPath, 'utf-8');

  it('sets min-width: 0 on .main so it can shrink below its flex-item content width', () => {
    const block = css.match(/\.main\s*\{[^}]*\}/s)?.[0] ?? '';

    // Without this, `.main` is a flex item of `.shell` (display: flex) and
    // keeps the flexbox default `min-width: auto`, which prevents it from
    // shrinking below its content's min-content width — causing horizontal
    // overflow on narrow viewports (e.g. /watch-zones/new at 390px).
    expect(block).toContain('min-width: 0');
  });
});
