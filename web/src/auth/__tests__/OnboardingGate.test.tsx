import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { ProfileRepositoryProvider } from '../profile-context.ts';
import { AuthProvider } from '../auth-context.ts';
import { OnboardingGate } from '../OnboardingGate.tsx';
import { SpyProfileRepository } from './spies/spy-profile-repository.ts';
import type { AuthPort } from '../../domain/ports/auth-port.ts';
import type { UserProfile } from '../../domain/types.ts';

const existingProfile: UserProfile = {
  userId: 'user-1',
  postcode: 'CB1 2AD',
  pushEnabled: false,
  tier: 'Free',
};

function stubAuth(overrides?: Partial<AuthPort>): AuthPort {
  return {
    isAuthenticated: true,
    isLoading: false,
    error: undefined,
    loginWithRedirect: async () => {},
    logout: async () => {},
    ...overrides,
  };
}

function renderWithProfile(spy: SpyProfileRepository, auth: AuthPort = stubAuth()) {
  return render(
    <MemoryRouter initialEntries={['/app']}>
      <AuthProvider value={auth}>
        <ProfileRepositoryProvider value={spy}>
          <Routes>
            <Route element={<OnboardingGate />}>
              <Route path="/app" element={<div>Dashboard</div>} />
            </Route>
            <Route path="/onboarding" element={<div>Onboarding</div>} />
          </Routes>
        </ProfileRepositoryProvider>
      </AuthProvider>
    </MemoryRouter>,
  );
}

describe('OnboardingGate', () => {
  it('renders child when profile exists', async () => {
    const spy = new SpyProfileRepository();
    spy.fetchProfileResult = existingProfile;

    renderWithProfile(spy);

    await waitFor(() => {
      expect(screen.getByText('Dashboard')).toBeInTheDocument();
    });
    expect(spy.fetchProfileCalls).toBe(1);
  });

  it('redirects to /onboarding when profile is null (404)', async () => {
    const spy = new SpyProfileRepository();
    spy.fetchProfileResult = null;

    renderWithProfile(spy);

    await waitFor(() => {
      expect(screen.getByText('Onboarding')).toBeInTheDocument();
    });
    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument();
  });

  it('shows loading indicator while loading', () => {
    // Create a spy that never resolves
    const spy = new SpyProfileRepository();
    spy.fetchProfileResult = existingProfile;
    // Override to return a never-resolving promise
    spy.fetchProfile = () => new Promise<UserProfile | null>(() => {});

    renderWithProfile(spy);

    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument();
    expect(screen.queryByText('Onboarding')).not.toBeInTheDocument();
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.getByText('Loading your profile…')).toBeInTheDocument();
  });

  it('shows error state when fetchProfile throws', async () => {
    const spy = new SpyProfileRepository();
    spy.fetchProfileError = new Error('token refresh failed');

    renderWithProfile(spy);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.getByText('Try again')).toBeInTheDocument();
    expect(screen.getByText('Sign out')).toBeInTheDocument();
    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument();
  });
});
