import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { FullPageLoader } from '../FullPageLoader';

describe('FullPageLoader', () => {
  it('renders with default message', () => {
    render(<FullPageLoader />);

    const status = screen.getByRole('status');
    expect(status).toBeInTheDocument();
    expect(status).toHaveAttribute('aria-label', 'Loading…');
    expect(screen.getByText('Loading…')).toBeInTheDocument();
  });

  it('renders with a custom message', () => {
    render(<FullPageLoader message="Signing you in…" />);

    const status = screen.getByRole('status');
    expect(status).toHaveAttribute('aria-label', 'Signing you in…');
    expect(screen.getByText('Signing you in…')).toBeInTheDocument();
  });
});
