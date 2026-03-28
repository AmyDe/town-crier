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
  it('shows nothing while loading', () => {
    const spy = new SpyAuthPort();
    spy.isLoading = true;

    const { container } = renderWithAuth(spy);

    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    expect(container.innerHTML).toBe('');
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
});
