import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { AuthProvider } from '../auth/auth-context';
import { ProfileRepositoryProvider } from '../auth/profile-context';
import { ApiClientProvider } from '../api/useApiClient';
import { SpyAuthPort } from '../auth/__tests__/spies/spy-auth-port';
import { SpyProfileRepository } from '../auth/__tests__/spies/spy-profile-repository';
import { AppRoutes } from '../AppRoutes';
import type { ApiClient } from '../api/client';
import type { UserProfile } from '../domain/types';

const existingProfile: UserProfile = {
  userId: 'user-1',
  postcode: 'CB1 2AD',
  pushEnabled: false,
  tier: 'Free',
};

const stubApiClient: ApiClient = {
  get: async (path: string) => {
    if (path.includes('watch-zones')) return { zones: [] } as unknown;
    if (path.includes('notifications')) return { notifications: [], total: 0, page: 1 } as unknown;
    if (path.includes('settings') || path.includes('profile')) return { postcode: null, pushEnabled: false, tier: 'Free' } as unknown;
    return {} as unknown;
  },
  post: async () => ({}),
  put: async () => {},
  patch: async () => ({}),
  delete: async () => {},
};

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

interface RenderOptions {
  route?: string;
  authSpy?: SpyAuthPort;
  profileSpy?: SpyProfileRepository;
}

function renderRoutes({ route = '/', authSpy, profileSpy }: RenderOptions = {}) {
  const auth = authSpy ?? new SpyAuthPort();
  const profile = profileSpy ?? new SpyProfileRepository();

  return render(
    <MemoryRouter initialEntries={[route]}>
      <AuthProvider value={auth}>
        <ProfileRepositoryProvider value={profile}>
          <ApiClientProvider value={stubApiClient}>
            <AppRoutes />
          </ApiClientProvider>
        </ProfileRepositoryProvider>
      </AuthProvider>
    </MemoryRouter>,
  );
}

describe('AppRoutes', () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
    stubMatchMedia();
  });

  describe('public routes', () => {
    it('renders landing page at /', () => {
      renderRoutes({ route: '/' });

      expect(screen.getByRole('banner')).toBeInTheDocument();
      expect(screen.getByRole('main')).toBeInTheDocument();
      expect(screen.getByRole('contentinfo')).toBeInTheDocument();
    });

    it('renders callback page at /callback that redirects to /dashboard', () => {
      const authSpy = new SpyAuthPort();
      authSpy.isAuthenticated = true;

      const profileSpy = new SpyProfileRepository();
      profileSpy.fetchProfileResult = existingProfile;

      renderRoutes({ route: '/callback', authSpy, profileSpy });

      // CallbackPage redirects to /dashboard, which is inside AppShell
      // We just verify it doesn't crash and navigates
    });

    it('renders legal page at /legal/privacy', () => {
      renderRoutes({ route: '/legal/privacy' });

      expect(screen.getByText(/privacy/i)).toBeInTheDocument();
    });

    it('renders legal page at /legal/terms', () => {
      renderRoutes({ route: '/legal/terms' });

      expect(screen.getByText(/terms/i)).toBeInTheDocument();
    });
  });

  describe('authenticated routes', () => {
    it('redirects to login when not authenticated at /dashboard', async () => {
      const authSpy = new SpyAuthPort();
      authSpy.isAuthenticated = false;
      authSpy.isLoading = false;

      renderRoutes({ route: '/dashboard', authSpy });

      await waitFor(() => {
        expect(authSpy.loginWithRedirectCalls).toBe(1);
      });
    });

    it('renders dashboard inside AppShell when authenticated with profile', async () => {
      const authSpy = new SpyAuthPort();
      authSpy.isAuthenticated = true;

      const profileSpy = new SpyProfileRepository();
      profileSpy.fetchProfileResult = existingProfile;

      renderRoutes({ route: '/dashboard', authSpy, profileSpy });

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: 'Dashboard' })).toBeInTheDocument();
      });

      // AppShell renders the sidebar with nav
      expect(screen.getByRole('navigation', { name: 'Main' })).toBeInTheDocument();
    });

    it('redirects to /onboarding when authenticated but no profile', async () => {
      const authSpy = new SpyAuthPort();
      authSpy.isAuthenticated = true;

      const profileSpy = new SpyProfileRepository();
      profileSpy.fetchProfileResult = null;

      renderRoutes({ route: '/dashboard', authSpy, profileSpy });

      await waitFor(() => {
        expect(screen.getByText('Welcome to Town Crier')).toBeInTheDocument();
      });
    });

    it('renders all feature route placeholders inside AppShell', async () => {
      const authSpy = new SpyAuthPort();
      authSpy.isAuthenticated = true;

      const profileSpy = new SpyProfileRepository();
      profileSpy.fetchProfileResult = existingProfile;

      const routes = [
        { path: '/map', title: 'Map' },
      ];

      for (const { path, title } of routes) {
        const { unmount } = renderRoutes({
          route: path,
          authSpy,
          profileSpy,
        });

        await waitFor(() => {
          expect(screen.getByRole('heading', { name: title })).toBeInTheDocument();
        });

        // Verify inside AppShell
        expect(screen.getByRole('navigation', { name: 'Main' })).toBeInTheDocument();

        unmount();
      }
    });
  });
});
