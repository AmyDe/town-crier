import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { Hero } from '../Hero';

describe('Hero', () => {
  it('renders the headline', () => {
    render(<Hero />);

    expect(
      screen.getByRole('heading', {
        level: 1,
        name: /stay informed about what's being built in your neighbourhood/i,
      }),
    ).toBeInTheDocument();
  });

  it('renders the subheading about 417 authorities', () => {
    render(<Hero />);

    expect(screen.getByText(/417 local authorities/i)).toBeInTheDocument();
  });

  it('renders an App Store CTA link', () => {
    render(<Hero />);

    const cta = screen.getByRole('link', { name: /app store/i });
    expect(cta).toBeInTheDocument();
    expect(cta).toHaveAttribute('href', expect.stringContaining('apps.apple.com'));
  });

  it('renders a scroll indicator', () => {
    render(<Hero />);

    expect(screen.getByLabelText(/scroll down/i)).toBeInTheDocument();
  });

  it('uses a section element with appropriate landmark', () => {
    render(<Hero />);

    const section = screen.getByRole('banner');
    expect(section).toBeInTheDocument();
  });
});
