import { describe, it, expect } from 'vitest';
import { buildShareUrl } from '../share-link';

describe('buildShareUrl', () => {
  it('builds the public share-page URL from an authority slug and reference', () => {
    expect(buildShareUrl('cambridge', '22/1234/FUL')).toBe(
      'https://share.towncrierapp.uk/a/cambridge/22/1234/FUL',
    );
  });

  it('does not URL-encode slashes in the reference', () => {
    // The share route (`GET /a/{authoritySlug}/{ref...}`) matches a trailing
    // wildcard on literal path segments — a PlanIt reference such as
    // "9/P/2026/0044/HH" must survive as raw slashes, not "%2F".
    const url = buildShareUrl('adur', '9/P/2026/0044/HH');
    expect(url).toBe('https://share.towncrierapp.uk/a/adur/9/P/2026/0044/HH');
    expect(url).not.toContain('%2F');
  });

  it('builds a URL for a reference with no slashes', () => {
    expect(buildShareUrl('cornwall', '24-0001-FUL')).toBe(
      'https://share.towncrierapp.uk/a/cornwall/24-0001-FUL',
    );
  });
});
