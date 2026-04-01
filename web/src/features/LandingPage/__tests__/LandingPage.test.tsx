import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { LandingPage } from '../LandingPage';

function stubMatchMedia(): void {
  const mediaQueryList: MediaQueryList = {
    matches: false,
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

function renderLandingPage() {
  return render(
    <MemoryRouter>
      <LandingPage />
    </MemoryRouter>,
  );
}

describe('LandingPage', () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    stubMatchMedia();
  });

  it('renders a navigation bar', () => {
    renderLandingPage();

    const navs = screen.getAllByRole('navigation');
    expect(navs.length).toBeGreaterThanOrEqual(1);
  });

  it('renders a hero banner', () => {
    renderLandingPage();

    expect(screen.getByRole('banner')).toBeInTheDocument();
  });

  it('renders a main content area with all sections', () => {
    renderLandingPage();

    const main = screen.getByRole('main');

    expect(within(main).getByText('How It Works')).toBeInTheDocument();
    expect(within(main).getByText('Pricing')).toBeInTheDocument();
    expect(within(main).getByText('Frequently Asked Questions')).toBeInTheDocument();
  });

  it('renders a footer', () => {
    renderLandingPage();

    expect(screen.getByRole('contentinfo')).toBeInTheDocument();
  });

  it('renders sections in correct order within main', () => {
    renderLandingPage();

    const main = screen.getByRole('main');
    const sectionIds = Array.from(main.querySelectorAll('section'))
      .map((section) => section.id)
      .filter(Boolean);

    expect(sectionIds).toEqual([
      'how-it-works',
      'pricing',
      'faq',
    ]);
  });
});
