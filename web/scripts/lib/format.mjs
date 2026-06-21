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
 * Truncate text to `maxLength` characters, appending a single ellipsis when
 * cut. Null/undefined coerce to an empty string.
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
  return text.slice(0, maxLength) + '…';
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
 * Collapse the server's raw per-`appState` distribution into resident-facing
 * labels. Each wire `appState` is translated via `statusDisplayLabel` and then
 * RE-AGGREGATED by that label, so null, "" and a literal "Unknown" all fold into
 * a single "Unknown" row whose count is the sum. Sorted most common first, then
 * alphabetically by label for a stable order.
 *
 * The breakdown is computed server-side over the bounded read (~200 docs), so it
 * can legitimately total more than the handful of cards rendered on the page.
 *
 * @param {ReadonlyArray<{ appState: string | null | undefined, count: number }>} statusBreakdown
 * @returns {LabelCount[]}
 */
export function aggregateBreakdown(statusBreakdown) {
  /** @type {Map<string, number>} */
  const counts = new Map();
  for (const { appState, count } of statusBreakdown) {
    const label = statusDisplayLabel(appState);
    counts.set(label, (counts.get(label) ?? 0) + count);
  }
  return [...counts.entries()]
    .map(([label, count]) => ({ label, count }))
    .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label));
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
