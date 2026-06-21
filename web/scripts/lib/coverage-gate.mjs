/**
 * Minimum number of recent applications an authority must have before we
 * publish a page for it. Guards against thin, auto-generated content that
 * Google's helpful-content system penalises.
 *
 * This is the implementable realisation of issue #570's "≥10 in the last
 * 90 days" intent: the slim SEO DTO does not expose `lastDifferent`, so the
 * `total` from the bounded most-recently-active read is the faithful proxy.
 * Strict 90-day windowing would need `lastDifferent` added to the DTO — a
 * future tweak, deliberately not built here.
 *
 * @type {number}
 */
export const COVERAGE_THRESHOLD = 10;

/**
 * @param {number} total count from the bounded recent-applications read
 * @returns {boolean} whether the authority clears the coverage gate
 */
export function meetsCoverageGate(total) {
  return total >= COVERAGE_THRESHOLD;
}
