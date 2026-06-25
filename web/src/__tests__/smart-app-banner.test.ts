import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { APPLE_APP_ID } from '../config/links';

const indexHtmlPath = resolve(__dirname, '../../index.html');
const html = readFileSync(indexHtmlPath, 'utf-8');

describe('Apple Smart App Banner (SPA shell)', () => {
  it('emits the apple-itunes-app meta tag in index.html', () => {
    expect(html).toContain(
      `<meta name="apple-itunes-app" content="app-id=${APPLE_APP_ID}" />`,
    );
    expect(html).toContain('app-id=6764095657');
  });

  it('places the banner meta inside <head>', () => {
    const head = html.match(/<head>[\s\S]*?<\/head>/)?.[0] ?? '';
    expect(head).toContain('name="apple-itunes-app"');
  });

  it('points at config/links.ts as the source of truth for the id', () => {
    // A comment keeps the static value honest against APPLE_APP_ID, which the
    // lockstep guard pins to constants.mjs. There is no banner without the note.
    expect(html).toContain('src/config/links.ts');
  });

  it('does not bake a campaign token into the banner (app-id only, no app-argument)', () => {
    expect(html).not.toContain('app-argument');
  });
});
