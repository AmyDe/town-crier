import { describe, it, expect } from 'vitest';
import { countBubbleHtml, statusPinHtml } from '../markerIcons';

describe('countBubbleHtml', () => {
  it('renders the aggregate count inside an amber bubble', () => {
    const html = countBubbleHtml(12);
    expect(html).toContain('tc-cluster-bubble');
    expect(html).toContain('12');
    expect(html).toContain('var(--tc-amber)');
  });
});

describe('statusPinHtml', () => {
  it('colours the pin with the matching status design token', () => {
    expect(statusPinHtml('Permitted')).toContain('var(--tc-status-permitted)');
    expect(statusPinHtml('Rejected')).toContain('var(--tc-status-rejected)');
    expect(statusPinHtml('Undecided')).toContain('var(--tc-status-pending)');
  });

  it('carries the status-pin class so CSS can theme it', () => {
    expect(statusPinHtml('Appealed')).toContain('tc-status-pin');
  });

  it('falls back to a neutral token for an unknown status', () => {
    expect(statusPinHtml('Something Else')).toContain('var(--tc-status-withdrawn)');
  });
});
