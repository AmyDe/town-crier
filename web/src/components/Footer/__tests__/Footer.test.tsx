import { render, screen, within } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { appStoreUrl } from '../../../config/links';
import { Footer } from '../Footer';

describe('Footer', () => {
  it('renders a footer element with id "footer"', () => {
    render(<Footer />);

    const footer = document.getElementById('footer');
    expect(footer).toBeInTheDocument();
    expect(footer!.tagName).toBe('FOOTER');
  });

  it('renders a CTA heading', () => {
    render(<Footer />);

    expect(
      screen.getByRole('heading', {
        name: /your neighbourhood is changing\. stay informed\./i,
      }),
    ).toBeInTheDocument();
  });

  it('renders a download link styled as a button pointing at the App Store', () => {
    render(<Footer />);

    const link = screen.getByRole('link', { name: /download on the app store/i });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute('href', appStoreUrl('web-home'));
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('renders copyright with the current year', () => {
    render(<Footer />);

    const currentYear = new Date().getFullYear().toString();
    expect(screen.getByText(new RegExp(`© ${currentYear} Town Crier`))).toBeInTheDocument();
  });

  it('discloses the operating company, place of registration, and company number', () => {
    render(<Footer />);

    expect(
      screen.getByText(
        /Ivo and the Bea Ltd · Registered in England & Wales · Company No\. 17222369/i,
      ),
    ).toBeInTheDocument();
  });

  it('renders a Privacy Policy link', () => {
    render(<Footer />);

    const link = screen.getByRole('link', { name: /privacy policy/i });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute('href', '/legal/privacy');
  });

  it('renders a Terms of Service link', () => {
    render(<Footer />);

    const link = screen.getByRole('link', { name: /terms of service/i });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute('href', '/legal/terms');
  });

  it('renders legal links inside a nav element for accessibility', () => {
    render(<Footer />);

    const nav = screen.getByRole('navigation', { name: /legal/i });
    expect(nav).toBeInTheDocument();

    const links = within(nav).getAllByRole('link');
    expect(links).toHaveLength(2);
  });

  describe('planning discovery links (GH #821 Phase 2)', () => {
    it('renders a link to the planning applications by council index', () => {
      render(<Footer />);

      const link = screen.getByRole('link', {
        name: /planning applications by council/i,
      });
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', '/planning/');
    });

    it('renders a link to the planning applications by town index', () => {
      render(<Footer />);

      const link = screen.getByRole('link', {
        name: /planning applications by town/i,
      });
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', '/planning/towns/');
    });

    it('groups the planning links inside a nav element for accessibility', () => {
      render(<Footer />);

      const nav = screen.getByRole('navigation', { name: /explore/i });
      expect(nav).toBeInTheDocument();

      const links = within(nav).getAllByRole('link');
      expect(links).toHaveLength(2);
    });

    it('does not yet link to /search (tc-geq7h.4 ships that separately)', () => {
      render(<Footer />);

      expect(
        screen.queryByRole('link', { name: /^search$/i }),
      ).not.toBeInTheDocument();
    });
  });
});
