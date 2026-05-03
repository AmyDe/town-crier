import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AuthProvider } from '../auth-context.ts';
import { AuthGuard } from '../AuthGuard.tsx';
import { SpyAuthPort } from './spies/spy-auth-port.ts';

function renderWithAuth(spy: SpyAuthPort) {
  return render(
    <MemoryRouter initialEntries={['/protected']}>
      <AuthProvider value={spy}>
        <Routes>
          <Route element={<AuthGuard />}>
            <Route path="/protected" element={<div>Protected Content</div>} />
          </Route>
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  );
}

describe('AuthGuard', () => {
  it('shows loading indicator while loading', () => {
    const spy = new SpyAuthPort();
    spy.isLoading = true;

    renderWithAuth(spy);

    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.getByText('Signing you in…')).toBeInTheDocument();
  });

  it('renders child when authenticated', () => {
    const spy = new SpyAuthPort();
    spy.isAuthenticated = true;

    renderWithAuth(spy);

    expect(screen.getByText('Protected Content')).toBeInTheDocument();
  });

  it('calls loginWithRedirect when not authenticated', async () => {
    const spy = new SpyAuthPort();
    spy.isAuthenticated = false;
    spy.isLoading = false;

    renderWithAuth(spy);

    await waitFor(() => {
      expect(spy.loginWithRedirectCalls).toBe(1);
    });
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
  });

  it('does not call loginWithRedirect when there is an auth error', () => {
    const spy = new SpyAuthPort();
    spy.isAuthenticated = false;
    spy.isLoading = false;
    spy.error = new Error('callback_failed');

    renderWithAuth(spy);

    expect(spy.loginWithRedirectCalls).toBe(0);
  });

  it('passes appState.returnTo with current pathname when redirecting to login', async () => {
    const spy = new SpyAuthPort();
    spy.isAuthenticated = false;
    spy.isLoading = false;

    render(
      <MemoryRouter initialEntries={['/applications/19/00123/FUL']}>
        <AuthProvider value={spy}>
          <Routes>
            <Route element={<AuthGuard />}>
              <Route path="/applications/*" element={<div>Detail</div>} />
            </Route>
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(spy.loginWithRedirectCalls).toBe(1);
    });
    expect(spy.lastLoginWithRedirectOptions).toEqual({
      appState: { returnTo: '/applications/19/00123/FUL' },
    });
  });
});
