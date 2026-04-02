import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { LoadingSkeleton } from '../LoadingSkeleton';

describe('LoadingSkeleton', () => {
  it('renders a loading skeleton with the correct aria role', () => {
    render(<LoadingSkeleton />);

    const skeleton = screen.getByRole('status');
    expect(skeleton).toBeInTheDocument();
    expect(skeleton).toHaveAttribute('aria-label', 'Loading');
  });

  it('renders multiple skeleton rows', () => {
    render(<LoadingSkeleton />);

    const rows = screen.getAllByTestId('skeleton-row');
    expect(rows.length).toBeGreaterThanOrEqual(3);
  });
});
