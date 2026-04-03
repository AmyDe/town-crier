import type { AuthorityListItem } from '../../../../domain/types';
import { asAuthorityId } from '../../../../domain/types';

export function cornwallAuthority(
  overrides?: Partial<AuthorityListItem>,
): AuthorityListItem {
  return {
    id: asAuthorityId(42),
    name: 'Cornwall Council',
    areaType: 'Unitary',
    ...overrides,
  };
}

export function bathAuthority(
  overrides?: Partial<AuthorityListItem>,
): AuthorityListItem {
  return {
    id: asAuthorityId(10),
    name: 'Bath and NE Somerset',
    areaType: 'Unitary',
    ...overrides,
  };
}
