import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
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

  it('does not render a call-to-action button when no handler is given', () => {
    render(<CustomShapeUpsell />);

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('renders a call-to-action button that invokes onUpgradeClick when clicked', async () => {
    const user = userEvent.setup();
    let clicked = 0;

    render(<CustomShapeUpsell onUpgradeClick={() => (clicked += 1)} />);

    const button = screen.getByRole('button', { name: /upgrade/i });
    await user.click(button);

    expect(clicked).toBe(1);
  });
});
