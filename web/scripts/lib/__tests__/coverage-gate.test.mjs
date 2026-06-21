import { describe, it, expect } from 'vitest';
import { COVERAGE_THRESHOLD, meetsCoverageGate } from '../coverage-gate.mjs';

describe('coverage gate', () => {
  it('publishes when total meets the threshold', () => {
    expect(meetsCoverageGate(COVERAGE_THRESHOLD)).toBe(true);
  });

  it('publishes when total exceeds the threshold', () => {
    expect(meetsCoverageGate(200)).toBe(true);
  });

  it('skips when total is one below the threshold', () => {
    expect(meetsCoverageGate(COVERAGE_THRESHOLD - 1)).toBe(false);
  });

  it('skips an authority with no applications', () => {
    expect(meetsCoverageGate(0)).toBe(false);
  });

  it('uses a threshold of ten', () => {
    expect(COVERAGE_THRESHOLD).toBe(10);
  });
});
