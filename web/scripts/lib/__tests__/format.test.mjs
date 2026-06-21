import { describe, it, expect } from 'vitest';
import {
  escapeHtml,
  truncate,
  formatDate,
  statusDisplayLabel,
  aggregateBreakdown,
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

  it('cuts long text and appends an ellipsis', () => {
    const long = 'a'.repeat(200);
    const result = truncate(long, 160);
    expect(result.length).toBe(161);
    expect(result.endsWith('…')).toBe(true);
  });

  it('treats null as empty', () => {
    expect(truncate(null, 160)).toBe('');
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

describe('aggregateBreakdown', () => {
  it('maps each raw appState to its resident label and re-aggregates by label', () => {
    const breakdown = [
      { appState: 'Permitted', count: 5 },
      { appState: 'Conditions', count: 2 },
      { appState: 'Rejected', count: 3 },
      { appState: null, count: 1 },
      { appState: '', count: 1 },
      { appState: 'Unknown', count: 2 },
    ];

    // null, '' and a literal "Unknown" all collapse to the "Unknown" label and
    // their counts sum (1 + 1 + 2 = 4). Sorted by count DESC, then label ASC.
    expect(aggregateBreakdown(breakdown)).toEqual([
      { label: 'Granted', count: 5 },
      { label: 'Unknown', count: 4 },
      { label: 'Refused', count: 3 },
      { label: 'Granted with conditions', count: 2 },
    ]);
  });

  it('breaks count ties alphabetically by label', () => {
    const breakdown = [
      { appState: 'Rejected', count: 2 },
      { appState: 'Permitted', count: 2 },
    ];

    expect(aggregateBreakdown(breakdown)).toEqual([
      { label: 'Granted', count: 2 },
      { label: 'Refused', count: 2 },
    ]);
  });

  it('returns an empty array for an empty breakdown', () => {
    expect(aggregateBreakdown([])).toEqual([]);
  });
});

describe('leadLine', () => {
  it('shows the exact total and the area name', () => {
    expect(leadLine('Adur', 42)).toBe(
      'Town Crier is tracking 42 planning applications in Adur.',
    );
  });

  it('uses the singular noun for a single application', () => {
    expect(leadLine('Tiny', 1)).toBe(
      'Town Crier is tracking 1 planning application in Tiny.',
    );
  });

  it('does not describe the count as "recent"', () => {
    expect(leadLine('Adur', 42)).not.toContain('recent');
  });
});
