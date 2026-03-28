import { Navigate } from 'react-router-dom';
import { useAuth } from './auth-context';

export function CallbackPage() {
  const { isLoading, isAuthenticated, error } = useAuth();

  if (isLoading) {
    return null;
  }

  if (error || !isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  return <Navigate to="/dashboard" replace />;
}
