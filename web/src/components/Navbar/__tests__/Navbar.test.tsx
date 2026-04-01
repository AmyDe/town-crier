import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { AuthProvider } from '../../../auth/auth-context';
import { SpyAuthPort } from '../../../auth/__tests__/spies/spy-auth-port';
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
  window.matchMedia = (() => mediaQueryList) as typeof window.matchMedia;
}

function renderNavbar(spy?: SpyAuthPort) {
  const authSpy = spy ?? new SpyAuthPort();
  return render(
    <MemoryRouter>
      <AuthProvider value={authSpy}>
        <Navbar />
      </AuthProvider>
    </MemoryRouter>,
  );
}

describe('Navbar', () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    stubMatchMedia(false);
  });

  it('renders a nav element', () => {
    renderNavbar();

    expect(screen.getByRole('navigation')).toBeInTheDocument();
  });

  it('renders logo link pointing to "#"', () => {
    renderNavbar();

    const logo = screen.getByRole('link', { name: /town crier/i });
    expect(logo).toHaveAttribute('href', '#');
  });

  it('renders anchor links for Features, Pricing, and FAQ', () => {
    renderNavbar();

    const nav = screen.getByRole('navigation');
    const features = within(nav).getByRole('link', { name: /features/i });
    const pricing = within(nav).getByRole('link', { name: /pricing/i });
    const faq = within(nav).getByRole('link', { name: /faq/i });

    expect(features).toHaveAttribute('href', '#how-it-works');
    expect(pricing).toHaveAttribute('href', '#pricing');
    expect(faq).toHaveAttribute('href', '#faq');
  });

  it('renders Download CTA link', () => {
    renderNavbar();

    const cta = screen.getByRole('link', { name: /download/i });
    expect(cta).toBeInTheDocument();
  });

  it('renders a ThemeToggle button', () => {
    renderNavbar();

    expect(
      screen.getByRole('button', { name: /switch to dark mode/i }),
    ).toBeInTheDocument();
  });

  it('renders a hamburger menu button', () => {
    renderNavbar();

    expect(
      screen.getByRole('button', { name: /menu/i }),
    ).toBeInTheDocument();
  });

  it('nav links are hidden by default on mobile (menu closed)', () => {
    renderNavbar();

    const navLinks = screen.getByTestId('nav-links');
    expect(navLinks).toHaveAttribute('data-open', 'false');
  });

  it('clicking hamburger opens the mobile menu', async () => {
    const user = userEvent.setup();
    renderNavbar();

    const menuButton = screen.getByRole('button', { name: /menu/i });
    await user.click(menuButton);

    const navLinks = screen.getByTestId('nav-links');
    expect(navLinks).toHaveAttribute('data-open', 'true');
  });

  it('clicking hamburger again closes the mobile menu', async () => {
    const user = userEvent.setup();
    renderNavbar();

    const menuButton = screen.getByRole('button', { name: /menu/i });
    await user.click(menuButton);
    await user.click(menuButton);

    const navLinks = screen.getByTestId('nav-links');
    expect(navLinks).toHaveAttribute('data-open', 'false');
  });

  it('hamburger button aria-expanded reflects menu state', async () => {
    const user = userEvent.setup();
    renderNavbar();

    const menuButton = screen.getByRole('button', { name: /menu/i });
    expect(menuButton).toHaveAttribute('aria-expanded', 'false');

    await user.click(menuButton);
    expect(menuButton).toHaveAttribute('aria-expanded', 'true');
  });

  describe('Sign In entry point', () => {
    it('renders a Sign In link to /dashboard when authenticated', () => {
      const spy = new SpyAuthPort();
      spy.isAuthenticated = true;

      renderNavbar(spy);

      const link = screen.getByRole('link', { name: /sign in/i });
      expect(link).toHaveAttribute('href', '/dashboard');
    });

    it('renders a Sign In button when not authenticated', () => {
      const spy = new SpyAuthPort();
      spy.isAuthenticated = false;

      renderNavbar(spy);

      const button = screen.getByRole('button', { name: /sign in/i });
      expect(button).toBeInTheDocument();
    });

    it('calls loginWithRedirect when Sign In button is clicked', async () => {
      const spy = new SpyAuthPort();
      spy.isAuthenticated = false;
      const user = userEvent.setup();

      renderNavbar(spy);

      const button = screen.getByRole('button', { name: /sign in/i });
      await user.click(button);

      expect(spy.loginWithRedirectCalls).toBe(1);
    });
  });
});
