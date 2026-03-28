import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { ProfileRepositoryProvider } from '../profile-context.ts';
import { OnboardingGate } from '../OnboardingGate.tsx';
import { SpyProfileRepository } from './spies/spy-profile-repository.ts';
import type { UserProfile } from '../../domain/types.ts';

const existingProfile: UserProfile = {
  userId: 'user-1',
  postcode: 'CB1 2AD',
  pushEnabled: false,
  tier: 'Free',
};

function renderWithProfile(spy: SpyProfileRepository) {
  return render(
    <MemoryRouter initialEntries={['/app']}>
      <ProfileRepositoryProvider value={spy}>
        <Routes>
          <Route element={<OnboardingGate />}>
            <Route path="/app" element={<div>Dashboard</div>} />
          </Route>
          <Route path="/onboarding" element={<div>Onboarding</div>} />
        </Routes>
      </ProfileRepositoryProvider>
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

  it('shows nothing while loading', () => {
    // Create a spy that never resolves
    const spy = new SpyProfileRepository();
    spy.fetchProfileResult = existingProfile;
    // Override to return a never-resolving promise
    spy.fetchProfile = () => new Promise<UserProfile | null>(() => {});

    const { container } = renderWithProfile(spy);

    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument();
    expect(screen.queryByText('Onboarding')).not.toBeInTheDocument();
    expect(container.innerHTML).toBe('');
  });
});
