import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { App } from '../App';

describe('App', () => {
  it('renders a Town Crier heading', () => {
    render(<App />);

    expect(
      screen.getByRole('heading', { name: /town crier/i }),
    ).toBeInTheDocument();
  });
});
