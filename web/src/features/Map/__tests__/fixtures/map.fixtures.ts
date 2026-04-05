import type { AuthorityListItem, PlanningApplication } from '../../../../domain/types';
import { asAuthorityId, asApplicationUid } from '../../../../domain/types';

export function anAuthority(overrides?: Partial<AuthorityListItem>): AuthorityListItem {
  return {
    id: asAuthorityId(1),
    name: 'Cambridge City Council',
    areaType: 'District',
    ...overrides,
  };
}

export function aSecondAuthority(overrides?: Partial<AuthorityListItem>): AuthorityListItem {
  return {
    id: asAuthorityId(2),
    name: 'Westminster City Council',
    areaType: 'London Borough',
    ...overrides,
  };
}

export function anApplication(overrides?: Partial<PlanningApplication>): PlanningApplication {
  return {
    name: 'Application 1',
    uid: asApplicationUid('app-001'),
    areaName: 'Cambridge City Council',
    areaId: asAuthorityId(1),
    address: '12 Mill Road, Cambridge',
    postcode: 'CB1 2AD',
    description: 'Erection of two-storey rear extension',
    appType: 'Full Planning',
    appState: 'Undecided',
    appSize: null,
    startDate: '2026-01-15',
    decidedDate: null,
    consultedDate: null,
    longitude: 0.1340,
    latitude: 52.1990,
    url: null,
    link: null,
    lastDifferent: '2026-01-10',
    ...overrides,
  };
}

export function aSecondApplication(overrides?: Partial<PlanningApplication>): PlanningApplication {
  return {
    name: 'Application 2',
    uid: asApplicationUid('app-002'),
    areaName: 'Westminster City Council',
    areaId: asAuthorityId(2),
    address: '45 High Street, London',
    postcode: 'SW1A 1AA',
    description: 'Change of use from retail to residential',
    appType: 'Change of Use',
    appState: 'Approved',
    appSize: null,
    startDate: '2026-02-01',
    decidedDate: '2026-03-01',
    consultedDate: null,
    longitude: -0.1357,
    latitude: 51.5014,
    url: null,
    link: null,
    lastDifferent: '2026-02-15',
    ...overrides,
  };
}
