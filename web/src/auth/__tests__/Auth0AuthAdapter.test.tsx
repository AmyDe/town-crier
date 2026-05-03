import { render, screen, act } from '@testing-library/react';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { Auth0AuthAdapter } from '../Auth0AuthAdapter';
import { captureAuth0RedirectReturnTo } from '../auth0-redirect-return-to';
import { useAuth } from '../auth-context';

const mockLogout = vi.fn<() => Promise<void>>();
const mockLoginWithRedirect = vi.fn<() => Promise<void>>();

vi.mock('@auth0/auth0-react', () => ({
  useAuth0: () => ({
    isAuthenticated: true,
    isLoading: false,
    error: undefined,
    loginWithRedirect: mockLoginWithRedirect,
    logout: mockLogout,
  }),
}));

function Consumer() {
  const auth = useAuth();
  return (
    <div>
      <span data-testid="authenticated">{String(auth.isAuthenticated)}</span>
      <span data-testid="return-to">{auth.returnTo ?? 'none'}</span>
      <button onClick={() => auth.logout()}>Sign Out</button>
    </div>
  );
}

describe('Auth0AuthAdapter', () => {
  beforeEach(() => {
    mockLogout.mockReset();
    mockLoginWithRedirect.mockReset();
    captureAuth0RedirectReturnTo(undefined);
  });

  it('provides isAuthenticated from Auth0', () => {
    render(
      <Auth0AuthAdapter>
        <Consumer />
      </Auth0AuthAdapter>,
    );

    expect(screen.getByTestId('authenticated')).toHaveTextContent('true');
  });

  it('calls auth0 logout with returnTo including signed_out param', async () => {
    render(
      <Auth0AuthAdapter>
        <Consumer />
      </Auth0AuthAdapter>,
    );

    await act(async () => {
      screen.getByRole('button', { name: /sign out/i }).click();
    });

    expect(mockLogout).toHaveBeenCalledTimes(1);
    expect(mockLogout).toHaveBeenCalledWith({
      logoutParams: {
        returnTo: `${window.location.origin}?signed_out=true`,
      },
    });
  });

  it('exposes returnTo captured from the Auth0 redirect callback', () => {
    captureAuth0RedirectReturnTo('/applications/19/00123/FUL');

    render(
      <Auth0AuthAdapter>
        <Consumer />
      </Auth0AuthAdapter>,
    );

    expect(screen.getByTestId('return-to')).toHaveTextContent(
      '/applications/19/00123/FUL',
    );
  });

  it('exposes undefined returnTo when no redirect callback has fired', () => {
    render(
      <Auth0AuthAdapter>
        <Consumer />
      </Auth0AuthAdapter>,
    );

    expect(screen.getByTestId('return-to')).toHaveTextContent('none');
  });
});
