/**
 * Formatting helpers shared by the planning-page renderer. All output is
 * destined for raw HTML, so callers MUST pass user/data-derived strings through
 * `escapeHtml` before interpolating them into markup.
 */

/**
 * Escape the five characters that can break out of HTML element or
 * double-quoted attribute context. Null/undefined coerce to an empty string.
 *
 * @param {string | null | undefined} value
 * @returns {string}
 */
export function escapeHtml(value) {
  if (value === null || value === undefined) {
    return '';
  }
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/**
 * Truncate text to at most `maxLength` characters, cutting on the last word
 * boundary within that limit so the result never ends mid-word, then
 * appending a single ellipsis. Falls back to a hard cut at `maxLength` when no
 * space exists within the limit (e.g. a single word longer than `maxLength`).
 * Null/undefined coerce to an empty string.
 *
 * @param {string | null | undefined} text
 * @param {number} maxLength
 * @returns {string}
 */
export function truncate(text, maxLength) {
  if (text === null || text === undefined) {
    return '';
  }
  if (text.length <= maxLength) {
    return text;
  }
  const cut = text.slice(0, maxLength);
  const lastSpace = cut.lastIndexOf(' ');
  const boundary = lastSpace > 0 ? cut.slice(0, lastSpace) : cut;
  return boundary + '…';
}

/**
 * Render a `yyyy-MM-dd` calendar date as en-GB short form ("15 Jan 2026").
 * Null/undefined or unparseable input yields an empty string.
 *
 * @param {string | null | undefined} isoDate
 * @returns {string}
 */
export function formatDate(isoDate) {
  if (isoDate === null || isoDate === undefined || isoDate === '') {
    return '';
  }
  const date = new Date(isoDate);
  if (Number.isNaN(date.getTime())) {
    return '';
  }
  return date.toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    timeZone: 'UTC',
  });
}

const STATUS_DISPLAY_LABEL_MAP = {
  Permitted: 'Granted',
  Conditions: 'Granted with conditions',
  Rejected: 'Refused',
};

/**
 * Translate a PlanIt `app_state` wire string into the resident-facing label.
 * Mirrors `src/utils/formatting.ts`. Null maps to "Unknown"; unknown states
 * pass through unchanged.
 *
 * @param {string | null | undefined} appState
 * @returns {string}
 */
export function statusDisplayLabel(appState) {
  if (appState === null || appState === undefined || appState === '') {
    return 'Unknown';
  }
  return STATUS_DISPLAY_LABEL_MAP[appState] ?? appState;
}

/**
 * @typedef {{ label: string, count: number }} LabelCount
 */

/**
 * @typedef {Object} StatusSummary
 * @property {number} granted
 * @property {number} refused
 * @property {number} undecided
 * @property {number} total
 * @property {LabelCount[]} other   long-tail states that don't fit the three
 *   headline buckets, most-common first then alphabetical by label — the
 *   caller folds these behind a disclosure instead of listing each as its own
 *   top-level row (tc-r4n9.3)
 */

/**
 * Collapse the server's raw per-`appState` distribution into the compact
 * three-bucket headline summary the "Status breakdown" strip needs
 * (Granted / Refused / Undecided + total), plus whatever doesn't fit those
 * three buckets so the caller can fold it behind a disclosure rather than
 * enumerate every long-tail state as its own top-level row (tc-r4n9.3,
 * punch-list #794 Phase 3).
 *
 * `Permitted` is granted and `Rejected` is refused. A `null`/absent state and
 * the literal `Undecided` wire value both mean no decision has been recorded
 * yet, so both join the "Undecided" headline bucket. Everything else — the
 * long tail (`Conditions`/"Granted with conditions", `Withdrawn`, `Appealed`,
 * `Referred`, `Unresolved`, any future/unrecognised state) — is decided-but-
 * different (or otherwise doesn't read as a plain yes/no/pending), so it is
 * aggregated by its resident label into `other` instead.
 *
 * @param {ReadonlyArray<{ appState: string | null | undefined, count: number }>} statusBreakdown
 * @returns {StatusSummary}
 */
export function aggregateStatusSummary(statusBreakdown) {
  let granted = 0;
  let refused = 0;
  let undecided = 0;
  let total = 0;
  /** @type {Map<string, number>} */
  const otherCounts = new Map();

  for (const { appState, count } of statusBreakdown) {
    total += count;
    if (appState === 'Permitted') {
      granted += count;
    } else if (appState === 'Rejected') {
      refused += count;
    } else if (appState === 'Undecided' || appState === null || appState === undefined || appState === '') {
      undecided += count;
    } else {
      const label = statusDisplayLabel(appState);
      otherCounts.set(label, (otherCounts.get(label) ?? 0) + count);
    }
  }

  const other = [...otherCounts.entries()]
    .map(([label, count]) => ({ label, count }))
    .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label));

  return { granted, refused, undecided, total, other };
}

/**
 * Find the most recent (max) `lastDifferent` among the applications actually
 * shown on a page and render it as the single "Data updated" line that
 * replaces the old per-card "Last updated {date}" repetition (tc-r4n9.3):
 * the same honest freshness signal the sitemap's lastmod uses (derived from
 * what's shown, not the build clock), surfaced once near the H1 instead of
 * once per card. Returns '' when no shown application carries a parseable
 * date, so the caller can omit the line rather than show a broken one.
 *
 * @param {ReadonlyArray<{ lastDifferent?: string | null }>} applications
 * @returns {string}
 */
export function dataUpdatedLine(applications) {
  let maxIso;
  let maxMs = -Infinity;
  for (const app of applications) {
    const iso = app?.lastDifferent;
    if (typeof iso !== 'string' || iso === '') {
      continue;
    }
    const ms = new Date(iso).getTime();
    if (!Number.isNaN(ms) && ms > maxMs) {
      maxMs = ms;
      maxIso = iso;
    }
  }
  if (maxIso === undefined) {
    return '';
  }
  return `Data updated ${formatDate(maxIso)}`;
}

/**
 * Build the data-filled lead sentence under the H1: the exact whole-partition
 * total with the right plural.
 *
 * @param {string} areaName
 * @param {number} total
 * @returns {string}
 */
export function leadLine(areaName, total) {
  const noun = total === 1 ? 'planning application' : 'planning applications';
  return `Town Crier is tracking ${total} ${noun} in ${areaName}.`;
}
