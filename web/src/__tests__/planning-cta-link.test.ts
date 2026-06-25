import { describe, it, expect } from 'vitest';
import { APP_DOWNLOAD_URL, APPLE_APP_ID, appStoreUrl } from '../config/links';
// The prerender build script is plain ESM (`.mjs`) and cannot import the app's
// TypeScript modules at runtime, so it keeps its own copy of the App Store URL,
// the numeric App Store id, and the campaign-tagging helper. These guards fail
// the build the moment the two drift apart.
import {
  APP_DOWNLOAD_URL as PRERENDER_APP_DOWNLOAD_URL,
  APPLE_APP_ID as PRERENDER_APPLE_APP_ID,
  appStoreUrl as prerenderAppStoreUrl,
} from '../../scripts/lib/constants.mjs';

describe('planning page CTA link', () => {
  it('keeps the prerender App Store URL in lockstep with config/links.ts', () => {
    expect(PRERENDER_APP_DOWNLOAD_URL).toBe(APP_DOWNLOAD_URL);
  });

  it('keeps the prerender App Store id in lockstep with config/links.ts', () => {
    expect(PRERENDER_APPLE_APP_ID).toBe(APPLE_APP_ID);
    // The id is the numeric tail of the download URL, never an independent value.
    expect(APP_DOWNLOAD_URL).toContain(APPLE_APP_ID);
  });

  it('keeps appStoreUrl() byte-identical between the two copies for the same campaign', () => {
    for (const campaign of ['seo-lpa', 'seo-town', 'web-home']) {
      expect(prerenderAppStoreUrl(campaign)).toBe(appStoreUrl(campaign));
    }
  });

  it('appStoreUrl() appends the campaign token and mt=8 to the campaign-free base', () => {
    expect(appStoreUrl('web-home')).toBe(`${APP_DOWNLOAD_URL}?ct=web-home&mt=8`);
    // The base constant itself stays campaign-free so the lockstep guard holds.
    expect(APP_DOWNLOAD_URL).not.toContain('ct=');
  });

  it('appStoreUrl() URL-encodes the campaign value', () => {
    expect(appStoreUrl('a b&c')).toBe(`${APP_DOWNLOAD_URL}?ct=a%20b%26c&mt=8`);
  });
});
