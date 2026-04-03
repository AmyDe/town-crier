import type { AuthorityListItem } from '../../../../domain/types';
import type { UserAuthoritiesPort } from '../../../../domain/ports/user-authorities-port';

export class SpyUserAuthoritiesPort implements UserAuthoritiesPort {
  fetchMyAuthoritiesCalls = 0;
  fetchMyAuthoritiesResult: readonly AuthorityListItem[] = [];
  fetchMyAuthoritiesError: Error | null = null;

  async fetchMyAuthorities(): Promise<readonly AuthorityListItem[]> {
    this.fetchMyAuthoritiesCalls++;
    if (this.fetchMyAuthoritiesError) {
      throw this.fetchMyAuthoritiesError;
    }
    return this.fetchMyAuthoritiesResult;
  }
}
