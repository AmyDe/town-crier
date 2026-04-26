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
    statusPermitted: 'statusPermitted_abc123',
    statusConditions: 'statusConditions_abc123',
    statusRejected: 'statusRejected_abc123',
    statusWithdrawn: 'statusWithdrawn_abc123',
    statusAppealed: 'statusAppealed_abc123',
    statusUnresolved: 'statusUnresolved_abc123',
    statusReferred: 'statusReferred_abc123',
    statusNotAvailable: 'statusNotAvailable_abc123',
    statusDefault: 'statusDefault_abc123',
  };

  it('returns the correct class for each PlanIt application state', () => {
    expect(statusClassName('Undecided', fakeStyles)).toBe('statusUndecided_abc123');
    expect(statusClassName('Permitted', fakeStyles)).toBe('statusPermitted_abc123');
    expect(statusClassName('Conditions', fakeStyles)).toBe('statusConditions_abc123');
    expect(statusClassName('Rejected', fakeStyles)).toBe('statusRejected_abc123');
    expect(statusClassName('Withdrawn', fakeStyles)).toBe('statusWithdrawn_abc123');
    expect(statusClassName('Appealed', fakeStyles)).toBe('statusAppealed_abc123');
    expect(statusClassName('Unresolved', fakeStyles)).toBe('statusUnresolved_abc123');
    expect(statusClassName('Referred', fakeStyles)).toBe('statusReferred_abc123');
    expect(statusClassName('Not Available', fakeStyles)).toBe('statusNotAvailable_abc123');
  });

  it('returns the statusDefault class for an unknown status', () => {
    expect(statusClassName('SomeUnknownStatus', fakeStyles)).toBe('statusDefault_abc123');
  });

  it('returns the statusDefault class for legacy "Approved" / "Refused" strings', () => {
    // PlanIt does not actually emit these strings — defensive fallback only.
    expect(statusClassName('Approved', fakeStyles)).toBe('statusDefault_abc123');
    expect(statusClassName('Refused', fakeStyles)).toBe('statusDefault_abc123');
  });

  it('returns an empty string when unknown status and no statusDefault in styles', () => {
    const stylesWithoutDefault: Record<string, string> = {
      statusUndecided: 'statusUndecided_abc123',
    };

    expect(statusClassName('SomeUnknownStatus', stylesWithoutDefault)).toBe('');
  });
});
