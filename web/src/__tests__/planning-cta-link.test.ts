import { describe, it, expect } from 'vitest';
import {
  APP_DOWNLOAD_URL,
  APPLE_APP_ID,
  APP_STORE_PROVIDER_TOKEN,
  appStoreUrl,
} from '../config/links';
describe('planning page CTA link', () => {
  it('keeps the App Store id as the numeric tail of the download URL', () => {
    expect(APP_DOWNLOAD_URL).toContain(APPLE_APP_ID);
  });

  it('appStoreUrl() appends the provider token, campaign token and mt=8 to the campaign-free base', () => {
    // Apple's App Analytics only records a ct campaign when pt is also present,
    // so the helper must always emit both.
    expect(appStoreUrl('web-home')).toBe(
      `${APP_DOWNLOAD_URL}?pt=${APP_STORE_PROVIDER_TOKEN}&ct=web-home&mt=8`,
    );
    expect(APP_DOWNLOAD_URL).not.toContain('ct=');
    expect(APP_DOWNLOAD_URL).not.toContain('pt=');
  });

  it('appStoreUrl() URL-encodes the campaign value', () => {
    expect(appStoreUrl('a b&c')).toBe(
      `${APP_DOWNLOAD_URL}?pt=${APP_STORE_PROVIDER_TOKEN}&ct=a%20b%26c&mt=8`,
    );
  });
});
