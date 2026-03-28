import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { CallbackPage } from '../CallbackPage.tsx';
import { AuthProvider } from '../auth-context.ts';
import type { AuthPort } from '../../domain/ports/auth-port.ts';

function renderWithAuth(authOverrides: Partial<AuthPort> = {}) {
  const auth: AuthPort = {
    isAuthenticated: false,
    isLoading: false,
    error: undefined,
    loginWithRedirect: vi.fn(),
    ...authOverrides,
  };

  return render(
    <AuthProvider value={auth}>
      <MemoryRouter initialEntries={['/callback']}>
        <Routes>
          <Route path="/callback" element={<CallbackPage />} />
          <Route path="/dashboard" element={<div>Dashboard</div>} />
          <Route path="/" element={<div>Landing</div>} />
        </Routes>
      </MemoryRouter>
    </AuthProvider>,
  );
}

describe('CallbackPage', () => {
  it('redirects to /dashboard when authenticated', () => {
    renderWithAuth({ isAuthenticated: true, isLoading: false });

    expect(screen.getByText('Dashboard')).toBeInTheDocument();
  });

  it('renders nothing while auth is loading', () => {
    const { container } = renderWithAuth({ isLoading: true });

    expect(container.innerHTML).toBe('');
  });

  it('redirects to landing page when not authenticated', () => {
    renderWithAuth({ isAuthenticated: false, isLoading: false });

    expect(screen.getByText('Landing')).toBeInTheDocument();
  });

  it('redirects to landing page on auth error', () => {
    renderWithAuth({
      isAuthenticated: false,
      isLoading: false,
      error: new Error('callback_failed'),
    });

    expect(screen.getByText('Landing')).toBeInTheDocument();
  });
});
