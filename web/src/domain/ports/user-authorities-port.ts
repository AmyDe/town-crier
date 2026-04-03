import type { AuthorityListItem } from '../types';

export interface UserAuthoritiesPort {
  fetchMyAuthorities(): Promise<readonly AuthorityListItem[]>;
}
