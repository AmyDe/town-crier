import { describe, it, expect } from 'vitest';
import {
  escapeHtml,
  truncate,
  formatDate,
  statusDisplayLabel,
  aggregateStatusSummary,
  dataUpdatedLine,
  leadLine,
} from '../format.mjs';

describe('escapeHtml', () => {
  it('escapes the characters that would break out of HTML/attribute context', () => {
    expect(escapeHtml(`<script>"a" & 'b'`)).toBe(
      '&lt;script&gt;&quot;a&quot; &amp; &#39;b&#39;',
    );
  });

  it('coerces null and undefined to an empty string', () => {
    expect(escapeHtml(null)).toBe('');
    expect(escapeHtml(undefined)).toBe('');
  });
});

describe('truncate', () => {
  it('returns short text unchanged', () => {
    expect(truncate('short', 160)).toBe('short');
  });

  it('treats null as empty', () => {
    expect(truncate(null, 160)).toBe('');
  });

  it('cuts on the last word boundary rather than mid-word', () => {
    const text =
      'Erection of a two-storey rear extension and associated landscaping works to an existing dwelling house with a new detached garage';
    const result = truncate(text, 60);
    // Every character up to the cut is a whole word from the source text — no
    // word is sliced in half.
    const withoutEllipsis = result.slice(0, -1);
    expect(text.startsWith(withoutEllipsis)).toBe(true);
    expect(withoutEllipsis.endsWith(' ')).toBe(false);
    const nextChar = text[withoutEllipsis.length];
    expect(nextChar === ' ' || nextChar === undefined).toBe(true);
    expect(result.endsWith('…')).toBe(true);
    expect(result.length).toBeLessThanOrEqual(61);
  });

  it('falls back to a hard cut when the maxLength falls inside the first word', () => {
    // No space within maxLength characters, so there is no word boundary to cut
    // on — this preserves the pre-existing hard-cut behaviour for that edge case.
    const long = 'a'.repeat(200);
    const result = truncate(long, 160);
    expect(result.length).toBe(161);
    expect(result.endsWith('…')).toBe(true);
  });
});

describe('formatDate', () => {
  it('renders a yyyy-MM-dd date in en-GB short form', () => {
    expect(formatDate('2026-01-15')).toBe('15 Jan 2026');
  });

  it('returns empty for a null date', () => {
    expect(formatDate(null)).toBe('');
  });
});

describe('statusDisplayLabel', () => {
  it('translates PlanIt wire states to resident-facing labels', () => {
    expect(statusDisplayLabel('Permitted')).toBe('Granted');
    expect(statusDisplayLabel('Conditions')).toBe('Granted with conditions');
    expect(statusDisplayLabel('Rejected')).toBe('Refused');
  });

  it('passes unknown states through unchanged', () => {
    expect(statusDisplayLabel('Undecided')).toBe('Undecided');
  });

  it('labels a null state as Unknown', () => {
    expect(statusDisplayLabel(null)).toBe('Unknown');
  });
});

describe('aggregateStatusSummary (tc-r4n9.3: compact Granted/Refused/Undecided strip)', () => {
  it('buckets Permitted as granted and Rejected as refused', () => {
    const breakdown = [
      { appState: 'Permitted', count: 20 },
      { appState: 'Rejected', count: 12 },
    ];
    expect(aggregateStatusSummary(breakdown)).toEqual({
      granted: 20,
      refused: 12,
      undecided: 0,
      total: 32,
      other: [],
    });
  });

  it('folds the literal "Undecided" wire state and a null/absent state together into the undecided headline bucket', () => {
    const breakdown = [
      { appState: 'Permitted', count: 20 },
      { appState: 'Rejected', count: 12 },
      { appState: 'Undecided', count: 8 },
      { appState: null, count: 2 },
    ];
    expect(aggregateStatusSummary(breakdown)).toEqual({
      granted: 20,
      refused: 12,
      undecided: 10,
      total: 42,
      other: [],
    });
  });

  it('folds long-tail decided-but-different states into "other", not the headline buckets', () => {
    const breakdown = [
      { appState: 'Permitted', count: 20 },
      { appState: 'Rejected', count: 12 },
      { appState: 'Conditions', count: 5 },
      { appState: 'Withdrawn', count: 3 },
      { appState: 'Referred', count: 1 },
    ];
    const summary = aggregateStatusSummary(breakdown);
    expect(summary.granted).toBe(20);
    expect(summary.refused).toBe(12);
    expect(summary.undecided).toBe(0);
    expect(summary.total).toBe(41);
    // Most-common first, ties broken alphabetically by label — same ordering
    // rule as the old per-card aggregation.
    expect(summary.other).toEqual([
      { label: 'Granted with conditions', count: 5 },
      { label: 'Withdrawn', count: 3 },
      { label: 'Referred', count: 1 },
    ]);
  });

  it('re-aggregates repeated long-tail labels (e.g. two raw states mapping to the same label) into one other row', () => {
    const breakdown = [
      { appState: 'Appealed', count: 2 },
      { appState: 'Unresolved', count: 1 },
    ];
    expect(aggregateStatusSummary(breakdown).other).toEqual([
      { label: 'Appealed', count: 2 },
      { label: 'Unresolved', count: 1 },
    ]);
  });

  it('returns all-zero buckets and an empty other list for an empty breakdown', () => {
    expect(aggregateStatusSummary([])).toEqual({
      granted: 0,
      refused: 0,
      undecided: 0,
      total: 0,
      other: [],
    });
  });
});

describe('dataUpdatedLine (tc-r4n9.3: single "Data updated" line replacing the per-card repetition)', () => {
  it('reports the freshest lastDifferent among the applications shown, formatted en-GB', () => {
    const applications = [
      { lastDifferent: '2026-06-12T09:30:00+00:00' },
      { lastDifferent: '2026-06-15T10:00:00+00:00' },
      { lastDifferent: '2026-06-09T08:00:00+00:00' },
    ];
    expect(dataUpdatedLine(applications)).toBe('Data updated 15 Jun 2026');
  });

  it('ignores applications with no parseable lastDifferent when finding the max', () => {
    const applications = [
      { lastDifferent: null },
      { lastDifferent: '2026-06-01T08:00:00+00:00' },
      { lastDifferent: '' },
    ];
    expect(dataUpdatedLine(applications)).toBe('Data updated 1 Jun 2026');
  });

  it('returns an empty string when no application carries a parseable date', () => {
    expect(dataUpdatedLine([{ lastDifferent: null }, { lastDifferent: '' }])).toBe('');
  });

  it('returns an empty string for an empty application list', () => {
    expect(dataUpdatedLine([])).toBe('');
  });
});

describe('leadLine (tc-r4n9.3: warmed-up one-sentence intro)', () => {
  it('shows the exact total and the area name', () => {
    expect(leadLine('Adur', 42)).toBe(
      "See what's happening with planning in Adur: 42 planning applications tracked so far.",
    );
  });

  it('uses the singular noun for a single application', () => {
    expect(leadLine('Tiny', 1)).toBe(
      "See what's happening with planning in Tiny: 1 planning application tracked so far.",
    );
  });

  it('does not describe the count as "recent"', () => {
    expect(leadLine('Adur', 42)).not.toContain('recent');
  });

  it('is a single sentence (one full stop, at the end)', () => {
    const line = leadLine('Adur', 42);
    expect(line.match(/\./g)).toHaveLength(1);
    expect(line.endsWith('.')).toBe(true);
  });

  it('never uses an em dash or en dash (voice skill hard rule)', () => {
    const line = leadLine('Adur', 42);
    expect(line).not.toContain('—');
    expect(line).not.toContain('–');
  });
});
