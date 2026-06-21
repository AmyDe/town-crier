import { describe, it, expect } from 'vitest';
import { APP_DOWNLOAD_URL } from '../config/links';
// The prerender build script is plain ESM (`.mjs`) and cannot import the app's
// TypeScript modules at runtime, so it keeps its own copy of the App Store URL.
// This guard fails the build the moment the two drift apart.
import { APP_DOWNLOAD_URL as PRERENDER_APP_DOWNLOAD_URL } from '../../scripts/lib/constants.mjs';

describe('planning page CTA link', () => {
  it('keeps the prerender App Store URL in lockstep with config/links.ts', () => {
    expect(PRERENDER_APP_DOWNLOAD_URL).toBe(APP_DOWNLOAD_URL);
  });
});
