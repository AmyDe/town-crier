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
 * @typedef {{ label: string, count: number }} StateCount
 */

/**
 * Count applications by resident-facing status label, most common first then
 * alphabetical for a stable order.
 *
 * @param {ReadonlyArray<{ appState: string | null | undefined }>} applications
 * @returns {StateCount[]}
 */
export function countByState(applications) {
  /** @type {Map<string, number>} */
  const counts = new Map();
  for (const app of applications) {
    const label = statusDisplayLabel(app.appState);
    counts.set(label, (counts.get(label) ?? 0) + 1);
  }
  return [...counts.entries()]
    .map(([label, count]) => ({ label, count }))
    .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label));
}

/**
 * Build the data-filled lead sentence under the H1. Uses "<total>+" when the
 * bounded read hit its cap, otherwise the exact total with the right plural.
 *
 * @param {string} areaName
 * @param {number} total
 * @param {boolean} totalCapped
 * @returns {string}
 */
export function leadLine(areaName, total, totalCapped) {
  const countPhrase = totalCapped ? `${total}+` : `${total}`;
  const noun =
    !totalCapped && total === 1
      ? 'recent planning application'
      : 'recent planning applications';
  return `Town Crier is tracking ${countPhrase} ${noun} in ${areaName}.`;
}
