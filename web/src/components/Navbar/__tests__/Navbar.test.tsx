import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, beforeEach } from 'vitest';
import { Navbar } from '../Navbar';

function stubMatchMedia(prefersDark: boolean): void {
  const mediaQueryList: MediaQueryList = {
    matches: prefersDark,
    media: '(prefers-color-scheme: dark)',
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  };
  window.matchMedia = (_query: string) => mediaQueryList;
}

describe('Navbar', () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    stubMatchMedia(false);
  });

  it('renders a nav element', () => {
    render(<Navbar />);

    expect(screen.getByRole('navigation')).toBeInTheDocument();
  });

  it('renders logo link pointing to "#"', () => {
    render(<Navbar />);

    const logo = screen.getByRole('link', { name: /town crier/i });
    expect(logo).toHaveAttribute('href', '#');
  });

  it('renders anchor links for Features, Pricing, and FAQ', () => {
    render(<Navbar />);

    const nav = screen.getByRole('navigation');
    const features = within(nav).getByRole('link', { name: /features/i });
    const pricing = within(nav).getByRole('link', { name: /pricing/i });
    const faq = within(nav).getByRole('link', { name: /faq/i });

    expect(features).toHaveAttribute('href', '#how-it-works');
    expect(pricing).toHaveAttribute('href', '#pricing');
    expect(faq).toHaveAttribute('href', '#faq');
  });

  it('renders Download CTA link', () => {
    render(<Navbar />);

    const cta = screen.getByRole('link', { name: /download/i });
    expect(cta).toBeInTheDocument();
  });

  it('renders a ThemeToggle button', () => {
    render(<Navbar />);

    expect(
      screen.getByRole('button', { name: /switch to dark mode/i }),
    ).toBeInTheDocument();
  });

  it('renders a hamburger menu button', () => {
    render(<Navbar />);

    expect(
      screen.getByRole('button', { name: /menu/i }),
    ).toBeInTheDocument();
  });

  it('nav links are hidden by default on mobile (menu closed)', () => {
    render(<Navbar />);

    const navLinks = screen.getByTestId('nav-links');
    expect(navLinks).toHaveAttribute('data-open', 'false');
  });

  it('clicking hamburger opens the mobile menu', async () => {
    const user = userEvent.setup();
    render(<Navbar />);

    const menuButton = screen.getByRole('button', { name: /menu/i });
    await user.click(menuButton);

    const navLinks = screen.getByTestId('nav-links');
    expect(navLinks).toHaveAttribute('data-open', 'true');
  });

  it('clicking hamburger again closes the mobile menu', async () => {
    const user = userEvent.setup();
    render(<Navbar />);

    const menuButton = screen.getByRole('button', { name: /menu/i });
    await user.click(menuButton);
    await user.click(menuButton);

    const navLinks = screen.getByTestId('nav-links');
    expect(navLinks).toHaveAttribute('data-open', 'false');
  });

  it('hamburger button aria-expanded reflects menu state', async () => {
    const user = userEvent.setup();
    render(<Navbar />);

    const menuButton = screen.getByRole('button', { name: /menu/i });
    expect(menuButton).toHaveAttribute('aria-expanded', 'false');

    await user.click(menuButton);
    expect(menuButton).toHaveAttribute('aria-expanded', 'true');
  });
});
