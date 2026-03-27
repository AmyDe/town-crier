import { createContext, useContext } from 'react';
import type { AuthPort } from '../domain/ports/auth-port.ts';

export const AuthContext = createContext<AuthPort | null>(null);

export function useAuth(): AuthPort {
  const ctx = useContext(AuthContext);
  if (ctx === null) {
    throw new Error('useAuth must be used within an AuthContext.Provider');
  }
  return ctx;
}
