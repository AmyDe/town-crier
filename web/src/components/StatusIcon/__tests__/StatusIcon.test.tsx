import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { StatusIcon } from '../StatusIcon';

describe('StatusIcon', () => {
  it('renders a decorative, screen-reader-hidden svg', () => {
    render(<StatusIcon appState="Permitted" />);

    const icon = screen.getByTestId('status-icon');
    expect(icon.tagName.toLowerCase()).toBe('svg');
    expect(icon).toHaveAttribute('aria-hidden', 'true');
  });

  it.each([
    ['Undecided', 'pending'],
    ['Unresolved', 'pending'],
    ['Not Available', 'pending'],
    ['Permitted', 'granted'],
    ['Conditions', 'granted'],
    ['Rejected', 'rejected'],
    ['Withdrawn', 'withdrawn'],
    ['Appealed', 'appealed'],
    ['Referred', 'appealed'],
  ])('maps appState %s to the %s icon glyph', (appState, expectedIcon) => {
    render(<StatusIcon appState={appState} />);

    expect(screen.getByTestId('status-icon')).toHaveAttribute('data-icon', expectedIcon);
  });

  it('falls back to the withdrawn glyph for an unrecognised appState', () => {
    render(<StatusIcon appState="Some Future State" />);

    expect(screen.getByTestId('status-icon')).toHaveAttribute('data-icon', 'withdrawn');
  });

  it('never renders visible text content (icon-only, paired label lives in the caller)', () => {
    render(<StatusIcon appState="Rejected" />);

    expect(screen.getByTestId('status-icon').textContent).toBe('');
  });

  it('accepts an additional className for layout composition by the caller', () => {
    render(<StatusIcon appState="Permitted" className="extra-class" />);

    expect(screen.getByTestId('status-icon').className).toMatch(/extra-class/);
  });
});
