import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, beforeEach } from 'vitest';
import { App } from '../App';

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

describe('App', () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    stubMatchMedia(false);
  });

  it('renders the primary navigation bar', () => {
    render(<App />);

    const navs = screen.getAllByRole('navigation');
    const primaryNav = navs.find((nav) => !nav.getAttribute('aria-label'));
    expect(primaryNav).toBeDefined();
  });

  it('renders a hero banner', () => {
    render(<App />);

    expect(screen.getByRole('banner')).toBeInTheDocument();
  });

  it('renders a main content area', () => {
    render(<App />);

    expect(screen.getByRole('main')).toBeInTheDocument();
  });

  it('renders a footer', () => {
    render(<App />);

    expect(screen.getByRole('contentinfo')).toBeInTheDocument();
  });

  it('renders all landing page sections inside main', () => {
    render(<App />);

    const main = screen.getByRole('main');

    expect(within(main).getByText('417')).toBeInTheDocument();
    expect(within(main).getByText('How It Works')).toBeInTheDocument();
    expect(within(main).getByText('Stronger together')).toBeInTheDocument();
    expect(within(main).getByText('Pricing')).toBeInTheDocument();
    expect(within(main).getByText('Frequently Asked Questions')).toBeInTheDocument();
  });

  it('renders sections in correct order within main', () => {
    render(<App />);

    const main = screen.getByRole('main');
    const sectionIds = Array.from(main.querySelectorAll('section'))
      .map((section) => section.id)
      .filter(Boolean);

    expect(sectionIds).toEqual([
      'how-it-works',
      'community-groups',
      'pricing',
      'faq',
    ]);
  });

  it('has anchor target ids matching navbar links', () => {
    render(<App />);

    expect(document.getElementById('how-it-works')).toBeInTheDocument();
    expect(document.getElementById('pricing')).toBeInTheDocument();
    expect(document.getElementById('faq')).toBeInTheDocument();
  });

  it('theme toggle switches data-theme attribute', async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(document.documentElement.getAttribute('data-theme')).toBe('light');

    const toggle = screen.getByRole('button', { name: /switch to dark mode/i });
    await user.click(toggle);

    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('navbar is before main in DOM order', () => {
    render(<App />);

    const navs = screen.getAllByRole('navigation');
    const primaryNav = navs[0]!;
    const main = screen.getByRole('main');

    const navPosition = primaryNav.compareDocumentPosition(main);
    expect(navPosition & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('footer is after main in DOM order', () => {
    render(<App />);

    const main = screen.getByRole('main');
    const footer = screen.getByRole('contentinfo');

    const mainPosition = main.compareDocumentPosition(footer);
    expect(mainPosition & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
