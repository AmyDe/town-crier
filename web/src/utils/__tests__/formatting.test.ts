import { describe, it, expect } from 'vitest';
import { formatDate, statusClassName, statusDisplayLabel } from '../formatting';

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

describe('statusDisplayLabel', () => {
  it('returns user-friendly UK planning vocabulary for decision states', () => {
    // Permitted/Conditions/Rejected are PlanIt wire strings;
    // residents talk about applications being "Granted" or "Refused".
    expect(statusDisplayLabel('Permitted')).toBe('Granted');
    expect(statusDisplayLabel('Conditions')).toBe('Granted with conditions');
    expect(statusDisplayLabel('Rejected')).toBe('Refused');
  });

  it('passes through non-decision states unchanged', () => {
    expect(statusDisplayLabel('Undecided')).toBe('Undecided');
    expect(statusDisplayLabel('Withdrawn')).toBe('Withdrawn');
    expect(statusDisplayLabel('Appealed')).toBe('Appealed');
    expect(statusDisplayLabel('Unresolved')).toBe('Unresolved');
    expect(statusDisplayLabel('Referred')).toBe('Referred');
    expect(statusDisplayLabel('Not Available')).toBe('Not Available');
  });

  it('returns the raw string for unknown values', () => {
    expect(statusDisplayLabel('SomethingElse')).toBe('SomethingElse');
  });
});
