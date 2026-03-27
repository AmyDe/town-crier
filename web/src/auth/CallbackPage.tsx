import { Navigate } from 'react-router-dom';

export function CallbackPage() {
  return <Navigate to="/dashboard" replace />;
}
