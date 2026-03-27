import type { AuthoritiesResult } from '../types';

export interface AuthoritySearchPort {
  search(query: string): Promise<AuthoritiesResult>;
}
