import type { ApiClient } from './client';
import { createRequiredContext } from '../utils/createRequiredContext.ts';

export const [ApiClientProvider, useApiClient] = createRequiredContext<ApiClient>('ApiClient');
