import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { ProGate } from '../ProGate';

describe('ProGate', () => {
  it('renders the upgrade heading', () => {
    render(<ProGate featureName="Advanced Filters" />);

    expect(
      screen.getByRole('heading', { name: /pro feature/i }),
    ).toBeInTheDocument();
  });

  it('displays the feature name in the message', () => {
    render(<ProGate featureName="Advanced Filters" />);

    expect(screen.getByText(/advanced filters/i)).toBeInTheDocument();
  });

  it('directs the user to the iOS app for upgrading', () => {
    render(<ProGate featureName="Advanced Filters" />);

    expect(
      screen.getByText(/ios app/i),
    ).toBeInTheDocument();
  });

  it('renders an accessible container role', () => {
    render(<ProGate featureName="Advanced Filters" />);

    expect(screen.getByRole('status')).toBeInTheDocument();
  });
});
