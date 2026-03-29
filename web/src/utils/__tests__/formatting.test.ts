import { describe, it, expect } from 'vitest';
import { formatDate } from '../formatting';

describe('formatDate', () => {
  it('formats an ISO date string as "day month year" in en-GB locale', () => {
    const result = formatDate('2026-01-15');

    expect(result).toBe('15 Jan 2026');
  });
});
