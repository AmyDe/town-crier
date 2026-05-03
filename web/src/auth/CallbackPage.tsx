import { Navigate } from 'react-router-dom';
import { useAuth } from './auth-context';

export function CallbackPage() {
  const { isLoading, isAuthenticated, error, returnTo } = useAuth();

  if (isLoading) {
    return null;
  }

  if (error) {
    console.error('[Auth0 callback failed]', error.message, error);
  }

  if (error || !isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  return <Navigate to={returnTo ?? '/dashboard'} replace />;
}
