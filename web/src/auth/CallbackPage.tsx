import { Navigate } from 'react-router-dom';
import { useAuth } from './auth-context';

export function CallbackPage() {
  const { isLoading } = useAuth();

  if (isLoading) {
    return null;
  }

  return <Navigate to="/dashboard" replace />;
}
