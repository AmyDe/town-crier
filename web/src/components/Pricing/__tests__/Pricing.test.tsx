import { render, screen, within } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { Pricing } from '../Pricing';

describe('Pricing', () => {
  it('renders a section with id="pricing"', () => {
    const { container } = render(<Pricing />);

    const section = container.querySelector('section#pricing');
    expect(section).toBeInTheDocument();
  });

  it('renders three pricing cards', () => {
    render(<Pricing />);

    const cards = screen.getAllByRole('article');
    expect(cards).toHaveLength(3);
  });

  it('renders Free tier with correct name and price', () => {
    render(<Pricing />);

    expect(screen.getByText('Free')).toBeInTheDocument();
    expect(screen.getByText('£0')).toBeInTheDocument();
  });

  it('renders Personal tier with correct name and price', () => {
    render(<Pricing />);

    expect(screen.getByText('Personal')).toBeInTheDocument();
    expect(screen.getByText('£1.99')).toBeInTheDocument();
  });

  it('renders Pro tier with correct name and price', () => {
    render(<Pricing />);

    expect(screen.getByText('Pro')).toBeInTheDocument();
    expect(screen.getByText('£5.99')).toBeInTheDocument();
  });

  it('shows "per month" text for paid tiers', () => {
    render(<Pricing />);

    const perMonthTexts = screen.getAllByText('/mo');
    expect(perMonthTexts).toHaveLength(2);
  });

  it('shows Recommended badge only on Personal tier', () => {
    render(<Pricing />);

    const badges = screen.getAllByText('Recommended');
    expect(badges).toHaveLength(1);

    // The badge should be within the Personal card
    const cards = screen.getAllByRole('article');
    const personalCard = cards.find((card) =>
      within(card).queryByText('Personal'),
    );
    expect(personalCard).toBeDefined();
    expect(within(personalCard!).getByText('Recommended')).toBeInTheDocument();
  });

  it('shows trial text for Personal tier', () => {
    render(<Pricing />);

    expect(screen.getByText(/free trial/i)).toBeInTheDocument();
  });

  it('renders feature rows with label/value pairs', () => {
    render(<Pricing />);

    expect(screen.getAllByText('Watch Zones').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Radius').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Notifications').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Search').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Historical Data').length).toBeGreaterThanOrEqual(1);
  });

  it('renders a section heading', () => {
    render(<Pricing />);

    expect(
      screen.getByRole('heading', { name: /pricing/i }),
    ).toBeInTheDocument();
  });
});
