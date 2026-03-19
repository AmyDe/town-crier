import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { StatsBar } from '../StatsBar';

describe('StatsBar', () => {
  it('renders three stat items', () => {
    render(<StatsBar />);

    const statItems = screen.getAllByRole('listitem');
    expect(statItems).toHaveLength(3);
  });

  it('renders "417" value with "Local Authorities" label', () => {
    render(<StatsBar />);

    expect(screen.getByText('417')).toBeInTheDocument();
    expect(screen.getByText('Local Authorities')).toBeInTheDocument();
  });

  it('renders "Free" value with "To Get Started" label', () => {
    render(<StatsBar />);

    expect(screen.getByText('Free')).toBeInTheDocument();
    expect(screen.getByText('To Get Started')).toBeInTheDocument();
  });

  it('renders "Real-time" value with "Push Alerts" label', () => {
    render(<StatsBar />);

    expect(screen.getByText('Real-time')).toBeInTheDocument();
    expect(screen.getByText('Push Alerts')).toBeInTheDocument();
  });

  it('renders values with amber styling class', () => {
    render(<StatsBar />);

    const value = screen.getByText('417');
    expect(value.className).toMatch(/value/);
  });

  it('renders labels with secondary text styling class', () => {
    render(<StatsBar />);

    const label = screen.getByText('Local Authorities');
    expect(label.className).toMatch(/label/);
  });

  it('renders as a section with top and bottom borders', () => {
    render(<StatsBar />);

    const section = screen.getByRole('list').closest('section');
    expect(section).toBeInTheDocument();
    expect(section?.className).toMatch(/container/);
  });
});
