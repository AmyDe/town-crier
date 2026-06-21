import { describe, it, expect } from 'vitest';
import {
  escapeHtml,
  truncate,
  formatDate,
  statusDisplayLabel,
  countByState,
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

describe('countByState', () => {
  it('counts applications by display label, most common first', () => {
    const apps = [
      { appState: 'Permitted' },
      { appState: 'Permitted' },
      { appState: 'Rejected' },
      { appState: null },
    ];

    expect(countByState(apps)).toEqual([
      { label: 'Granted', count: 2 },
      { label: 'Refused', count: 1 },
      { label: 'Unknown', count: 1 },
    ]);
  });
});

describe('leadLine', () => {
  it('shows the exact total when the read was not capped', () => {
    expect(leadLine('Adur', 42, false)).toContain('42');
    expect(leadLine('Adur', 42, false)).toContain('Adur');
  });

  it('shows a capped count as "<total>+" when the read hit the cap', () => {
    expect(leadLine('Leeds', 200, true)).toContain('200+');
  });

  it('uses the singular noun for a single application', () => {
    expect(leadLine('Tiny', 1, false)).toContain('1 recent planning application ');
  });
});
