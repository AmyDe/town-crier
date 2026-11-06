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
    expect(screen.getByText('£4.99')).toBeInTheDocument();
    expect(screen.queryByText('£5.99')).not.toBeInTheDocument();
  });

  it('shows the yearly and lifetime prices on the Pro card', () => {
    render(<Pricing />);

    const cards = screen.getAllByRole('article');
    const proCard = cards.find((card) => within(card).queryByText('Pro'));
    expect(proCard).toBeDefined();

    expect(within(proCard!).getByText('£29.99')).toBeInTheDocument();
    expect(within(proCard!).getByText('/yr')).toBeInTheDocument();
    expect(within(proCard!).getByText('£69.99')).toBeInTheDocument();
    expect(within(proCard!).getByText('once, for life')).toBeInTheDocument();
    expect(
      within(proCard!).getByText('Yearly and lifetime plans are in the iOS app.'),
    ).toBeInTheDocument();
  });

  it('shows yearly and lifetime prices only on the Pro card', () => {
    render(<Pricing />);

    expect(screen.getAllByText('/yr')).toHaveLength(1);
    expect(screen.getAllByText(/in the iOS app/i)).toHaveLength(1);
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

  it('shows 7-day free trial (not 14-day) on Personal tier', () => {
    render(<Pricing />);

    expect(screen.getByText('7-day free trial')).toBeInTheDocument();
  });

  it('shows Personal watch zone quota as 3 (matching EntitlementMap)', () => {
    render(<Pricing />);

    const cards = screen.getAllByRole('article');
    const personalCard = cards.find((card) => within(card).queryByText('Personal'));
    expect(personalCard).toBeDefined();

    const watchZoneRow = within(personalCard!).getByText('Watch Zones').closest('li');
    expect(watchZoneRow).toBeDefined();
    expect(within(watchZoneRow!).getByText('3')).toBeInTheDocument();
  });

  it('renders feature rows with label/value pairs', () => {
    render(<Pricing />);

    expect(screen.getAllByText('Watch Zones')).toHaveLength(3);
    expect(screen.getAllByText('Radius')).toHaveLength(3);
    expect(screen.getAllByText('Custom Shapes')).toHaveLength(3);
    expect(screen.getAllByText('Notifications')).toHaveLength(3);
  });

  it('does not list search or history as tier features', () => {
    render(<Pricing />);

    expect(screen.queryByText('Search')).not.toBeInTheDocument();
    expect(screen.queryByText('Historical Data')).not.toBeInTheDocument();
  });

  it('shows the radius each tier allows (matching WatchZoneLimits)', () => {
    render(<Pricing />);

    const cards = screen.getAllByRole('article');
    const radiusFor = (tier: string) => {
      const card = cards.find((c) => within(c).queryByText(tier));
      return within(card!).getByText('Radius').closest('li');
    };

    expect(within(radiusFor('Free')!).getByText('2km')).toBeInTheDocument();
    expect(within(radiusFor('Personal')!).getByText('5km')).toBeInTheDocument();
    expect(within(radiusFor('Pro')!).getByText('10km')).toBeInTheDocument();
  });

  it('shows custom shapes as circle only on Free and available on Personal and Pro', () => {
    render(<Pricing />);

    const cards = screen.getAllByRole('article');
    const freeCard = cards.find((card) => within(card).queryByText('Free'));
    const personalCard = cards.find((card) => within(card).queryByText('Personal'));
    const proCard = cards.find((card) => within(card).queryByText('Pro'));
    expect(freeCard).toBeDefined();
    expect(personalCard).toBeDefined();
    expect(proCard).toBeDefined();

    const freeRow = within(freeCard!).getByText('Custom Shapes').closest('li');
    const personalRow = within(personalCard!).getByText('Custom Shapes').closest('li');
    const proRow = within(proCard!).getByText('Custom Shapes').closest('li');
    expect(freeRow).toBeDefined();
    expect(personalRow).toBeDefined();
    expect(proRow).toBeDefined();

    expect(within(freeRow!).getByText('Circle only')).toBeInTheDocument();
    expect(within(personalRow!).getByText('Yes')).toBeInTheDocument();
    expect(within(proRow!).getByText('Yes')).toBeInTheDocument();
  });

  it('renders a section heading', () => {
    render(<Pricing />);

    expect(
      screen.getByRole('heading', { name: /pricing/i }),
    ).toBeInTheDocument();
  });

  it('renders a filled amber CTA only on the recommended (paid) tier card', () => {
    render(<Pricing />);

    const cards = screen.getAllByRole('article');
    const ctas = screen.getAllByRole('link', { name: /start.*trial/i });
    expect(ctas).toHaveLength(1);

    const personalCard = cards.find((card) => within(card).queryByText('Personal'));
    expect(personalCard).toBeDefined();
    expect(within(personalCard!).getByRole('link', { name: /start.*trial/i })).toBeInTheDocument();
  });

  it('does not render a CTA on the Free or Pro tier cards', () => {
    render(<Pricing />);

    const cards = screen.getAllByRole('article');
    const freeCard = cards.find((card) => within(card).queryByText('Free'));
    const proCard = cards.find((card) => within(card).queryByText('Pro'));
    expect(freeCard).toBeDefined();
    expect(proCard).toBeDefined();

    expect(within(freeCard!).queryByRole('link')).not.toBeInTheDocument();
    expect(within(proCard!).queryByRole('link')).not.toBeInTheDocument();
  });
});
