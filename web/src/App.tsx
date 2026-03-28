import { BrowserRouter } from 'react-router-dom';
import { Auth0Provider } from '@auth0/auth0-react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { loadAuthConfig } from './auth/auth-config';
import { Auth0AuthAdapter } from './auth/Auth0AuthAdapter';
import { ApiClientProvider } from './api/ApiClientProvider';
import { ProfileRepositoryProvider } from './auth/ProfileRepositoryProvider';
import { AppRoutes } from './AppRoutes';

const authConfig = loadAuthConfig();
const queryClient = new QueryClient();

export function App() {
  return (
    <Auth0Provider
      domain={authConfig.domain}
      clientId={authConfig.clientId}
      authorizationParams={{
        redirect_uri: `${window.location.origin}/callback`,
        audience: authConfig.audience,
      }}
      useRefreshTokens
      cacheLocation="localstorage"
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
  );
}
