import type { AuthorityListItem, AuthoritiesResult } from '../../../../domain/types';
import { asAuthorityId } from '../../../../domain/types';

export function cambridgeAuthority(
  overrides?: Partial<AuthorityListItem>,
): AuthorityListItem {
  return {
    id: asAuthorityId(101),
    name: 'Cambridge City Council',
    areaType: 'London Borough',
    ...overrides,
  };
}

export function oxfordAuthority(
  overrides?: Partial<AuthorityListItem>,
): AuthorityListItem {
  return {
    id: asAuthorityId(202),
    name: 'Oxford City Council',
    areaType: 'District',
    ...overrides,
  };
}

export function twoAuthorityResults(): AuthoritiesResult {
  return {
    authorities: [cambridgeAuthority(), oxfordAuthority()],
    total: 2,
  };
}
