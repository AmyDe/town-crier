import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { CustomShapeUpsell } from '../CustomShapeUpsell';

describe('CustomShapeUpsell', () => {
  it('renders the heading and value-prop copy', () => {
    render(<CustomShapeUpsell />);

    expect(
      screen.getByRole('heading', { name: /draw any shape/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/exact area you care about/i)).toBeInTheDocument();
  });

  it('renders an inline SVG graphic', () => {
    const { container } = render(<CustomShapeUpsell />);

    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveAttribute('aria-hidden', 'true');
  });

  it('does not render a call-to-action link when no upgradeHref is given', () => {
    render(<CustomShapeUpsell />);

    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('renders a call-to-action link pointing at upgradeHref, opening in a new tab', () => {
    render(<CustomShapeUpsell upgradeHref="https://apps.apple.com/gb/app/town-crier-planning-alerts/id6764095657?ct=test" />);

    const link = screen.getByRole('link', { name: /upgrade/i });
    expect(link).toHaveAttribute(
      'href',
      'https://apps.apple.com/gb/app/town-crier-planning-alerts/id6764095657?ct=test',
    );
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });
});
