import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect } from 'vitest';
import { AuthProvider } from '../../../auth/auth-context';
import { SpyAuthPort } from '../../../auth/__tests__/spies/spy-auth-port';
import { Hero } from '../Hero';

function renderHero(spy?: SpyAuthPort) {
  const authSpy = spy ?? new SpyAuthPort();
  return render(
    <AuthProvider value={authSpy}>
      <Hero />
    </AuthProvider>,
  );
}

describe('Hero', () => {
  it('renders the headline', () => {
    renderHero();

    expect(
      screen.getByRole('heading', {
        level: 1,
        name: /stay informed about what's being built in your neighbourhood/i,
      }),
    ).toBeInTheDocument();
  });

  it('renders the subheading about 417 authorities', () => {
    renderHero();

    expect(screen.getByText(/417 local authorities/i)).toBeInTheDocument();
  });

  it('renders an App Store CTA link', () => {
    renderHero();

    const cta = screen.getByRole('link', { name: /app store/i });
    expect(cta).toBeInTheDocument();
    expect(cta).toHaveAttribute('href', expect.stringContaining('apps.apple.com'));
  });

  it('renders a scroll indicator', () => {
    renderHero();

    expect(screen.getByLabelText(/scroll down/i)).toBeInTheDocument();
  });

  it('uses a section element with appropriate landmark', () => {
    renderHero();

    const section = screen.getByRole('banner');
    expect(section).toBeInTheDocument();
  });

  describe('Try the Web App CTA', () => {
    it('renders a Try the Web App button', () => {
      renderHero();

      const button = screen.getByRole('button', { name: /try the web app/i });
      expect(button).toBeInTheDocument();
    });

    it('calls loginWithRedirect when Try the Web App is clicked', async () => {
      const spy = new SpyAuthPort();
      const user = userEvent.setup();

      renderHero(spy);

      const button = screen.getByRole('button', { name: /try the web app/i });
      await user.click(button);

      expect(spy.loginWithRedirectCalls).toBe(1);
    });

    it('keeps the App Store link as the primary CTA', () => {
      renderHero();

      const appStoreLink = screen.getByRole('link', { name: /app store/i });
      const webAppButton = screen.getByRole('button', { name: /try the web app/i });

      expect(appStoreLink).toBeInTheDocument();
      expect(webAppButton).toBeInTheDocument();
    });
  });
});
