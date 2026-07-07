import { describe, it, expect } from 'vitest';
import {
  APP_DOWNLOAD_URL,
  APPLE_APP_ID,
  APP_STORE_PROVIDER_TOKEN,
  appStoreUrl,
} from '../config/links';
// The prerender build script is plain ESM (`.mjs`) and cannot import the app's
// TypeScript modules at runtime, so it keeps its own copy of the App Store URL,
// the numeric App Store id, and the campaign-tagging helper. These guards fail
// the build the moment the two drift apart.
import {
  APP_DOWNLOAD_URL as PRERENDER_APP_DOWNLOAD_URL,
  APPLE_APP_ID as PRERENDER_APPLE_APP_ID,
  APP_STORE_PROVIDER_TOKEN as PRERENDER_PROVIDER_TOKEN,
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

  it('keeps the provider token in lockstep with config/links.ts', () => {
    expect(PRERENDER_PROVIDER_TOKEN).toBe(APP_STORE_PROVIDER_TOKEN);
  });

  it('keeps appStoreUrl() byte-identical between the two copies for the same campaign', () => {
    for (const campaign of ['seo-lpa-inline', 'seo-town-btm', 'web-home']) {
      expect(prerenderAppStoreUrl(campaign)).toBe(appStoreUrl(campaign));
    }
  });

  it('appStoreUrl() appends the provider token, campaign token and mt=8 to the campaign-free base', () => {
    // Apple's App Analytics only records a ct campaign when pt is also present,
    // so the helper must always emit both.
    expect(appStoreUrl('web-home')).toBe(
      `${APP_DOWNLOAD_URL}?pt=${APP_STORE_PROVIDER_TOKEN}&ct=web-home&mt=8`,
    );
    // The base constant itself stays campaign-free so the lockstep guard holds.
    expect(APP_DOWNLOAD_URL).not.toContain('ct=');
    expect(APP_DOWNLOAD_URL).not.toContain('pt=');
  });

  it('appStoreUrl() URL-encodes the campaign value', () => {
    expect(appStoreUrl('a b&c')).toBe(
      `${APP_DOWNLOAD_URL}?pt=${APP_STORE_PROVIDER_TOKEN}&ct=a%20b%26c&mt=8`,
    );
  });
});
