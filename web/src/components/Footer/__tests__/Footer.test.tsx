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

  it('renders small-caps section labels for Explore and Legal', () => {
    render(<Footer />);

    expect(screen.getByText('Explore')).toBeInTheDocument();
    expect(screen.getByText('Legal')).toBeInTheDocument();
  });

  it('renders an Explore nav linking to the planning authority and town indexes plus search', () => {
    render(<Footer />);

    const nav = screen.getByRole('navigation', { name: /explore/i });
    expect(nav).toBeInTheDocument();

    const authorityLink = within(nav).getByRole('link', { name: /planning applications by council/i });
    expect(authorityLink).toHaveAttribute('href', '/planning/');

    const townLink = within(nav).getByRole('link', { name: /planning applications by town/i });
    expect(townLink).toHaveAttribute('href', '/planning/towns/');

    const searchLink = within(nav).getByRole('link', { name: /search planning applications/i });
    expect(searchLink).toHaveAttribute('href', '/search');

    expect(within(nav).getAllByRole('link')).toHaveLength(3);
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

    it('groups the planning and search links inside a nav element for accessibility', () => {
      render(<Footer />);

      const nav = screen.getByRole('navigation', { name: /explore/i });
      expect(nav).toBeInTheDocument();

      const links = within(nav).getAllByRole('link');
      expect(links).toHaveLength(3);
    });
  });
});
