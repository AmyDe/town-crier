import { describe, it, expect } from 'vitest';
import { SHARE_ORIGIN, shareUrl } from '../constants.mjs';

describe('shareUrl', () => {
  it('builds the canonical /a/{slug}/{ref} share URL', () => {
    expect(shareUrl('croydon', '23/03456/FUL')).toBe(
      `${SHARE_ORIGIN}/a/croydon/23/03456/FUL`,
    );
  });

  it('keeps ref slashes as path separators but encodes other unsafe characters', () => {
    // A space and an ampersand are percent-encoded; the slashes are preserved.
    expect(shareUrl('cornwall', 'PA/2026/00123 A&B')).toBe(
      `${SHARE_ORIGIN}/a/cornwall/PA/2026/00123%20A%26B`,
    );
    // A '?' or '#' in a ref must be encoded so it cannot truncate the path.
    expect(shareUrl('cornwall', 'PA?2026#1')).toBe(
      `${SHARE_ORIGIN}/a/cornwall/PA%3F2026%231`,
    );
  });

  it('returns null when the slug or ref is missing, so callers can omit the link', () => {
    expect(shareUrl('', '23/03456/FUL')).toBeNull();
    expect(shareUrl('croydon', '')).toBeNull();
    expect(shareUrl(undefined, undefined)).toBeNull();
  });
});
