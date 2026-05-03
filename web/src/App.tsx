import { BrowserRouter } from 'react-router-dom';
import { Auth0Provider, type AppState } from '@auth0/auth0-react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { loadAuthConfig } from './auth/auth-config';
import { Auth0AuthAdapter } from './auth/Auth0AuthAdapter';
import { captureAuth0RedirectReturnTo } from './auth/auth0-redirect-return-to';
import { ApiClientProvider } from './api/ApiClientProvider';
import { ProfileRepositoryProvider } from './auth/ProfileRepositoryProvider';
import { AppRoutes } from './AppRoutes';
import { ErrorBoundary } from './components/ErrorBoundary';

const authConfig = loadAuthConfig();
const queryClient = new QueryClient();

function handleRedirectCallback(appState?: AppState): void {
  const returnTo =
    appState && typeof appState.returnTo === 'string' ? appState.returnTo : undefined;
  captureAuth0RedirectReturnTo(returnTo);
}

export function App() {
  return (
    <ErrorBoundary>
      <Auth0Provider
        domain={authConfig.domain}
        clientId={authConfig.clientId}
        authorizationParams={{
          redirect_uri: `${window.location.origin}/callback`,
          audience: authConfig.audience,
        }}
        useRefreshTokens
        cacheLocation="localstorage"
        onRedirectCallback={handleRedirectCallback}
      >
        <QueryClientProvider client={queryClient}>
          <Auth0AuthAdapter>
            <ApiClientProvider>
              <ProfileRepositoryProvider>
                <BrowserRouter>
                  <AppRoutes />
                </BrowserRouter>
              </ProfileRepositoryProvider>
            </ApiClientProvider>
          </Auth0AuthAdapter>
        </QueryClientProvider>
      </Auth0Provider>
    </ErrorBoundary>
  );
}
