import type { AuthPort } from '../domain/ports/auth-port.ts';
import { createRequiredContext } from '../utils/createRequiredContext.ts';

export const [AuthProvider, useAuth] = createRequiredContext<AuthPort>('AuthContext');
