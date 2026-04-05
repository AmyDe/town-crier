import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { Toast } from '../Toast';

describe('Toast', () => {
  it('renders the message', () => {
    render(<Toast message="You've been signed out" onDismiss={() => {}} />);

    expect(screen.getByText("You've been signed out")).toBeInTheDocument();
  });
});
