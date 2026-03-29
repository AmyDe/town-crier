import { describe, it, expect } from 'vitest';
import { formatDate, statusClassName } from '../formatting';

describe('formatDate', () => {
  it('formats an ISO date string as "day month year" in en-GB locale', () => {
    const result = formatDate('2026-01-15');

    expect(result).toBe('15 Jan 2026');
  });
});

describe('statusClassName', () => {
  const fakeStyles: Record<string, string> = {
    statusUndecided: 'statusUndecided_abc123',
    statusApproved: 'statusApproved_abc123',
    statusRefused: 'statusRefused_abc123',
    statusWithdrawn: 'statusWithdrawn_abc123',
    statusAppealed: 'statusAppealed_abc123',
    statusNotAvailable: 'statusNotAvailable_abc123',
    statusDefault: 'statusDefault_abc123',
  };

  it('returns the correct class for each known application status', () => {
    expect(statusClassName('Undecided', fakeStyles)).toBe('statusUndecided_abc123');
    expect(statusClassName('Approved', fakeStyles)).toBe('statusApproved_abc123');
    expect(statusClassName('Refused', fakeStyles)).toBe('statusRefused_abc123');
    expect(statusClassName('Withdrawn', fakeStyles)).toBe('statusWithdrawn_abc123');
    expect(statusClassName('Appealed', fakeStyles)).toBe('statusAppealed_abc123');
    expect(statusClassName('Not Available', fakeStyles)).toBe('statusNotAvailable_abc123');
  });
});
