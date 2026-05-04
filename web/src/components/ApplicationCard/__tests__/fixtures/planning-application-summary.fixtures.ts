import type { PlanningApplicationSummary } from '../../../../domain/types';
import { asApplicationUid } from '../../../../domain/types';

export function undecidedApplication(
  overrides?: Partial<PlanningApplicationSummary>,
): PlanningApplicationSummary {
  return {
    uid: asApplicationUid('APP-001'),
    name: '2026/0042/FUL',
    address: '12 Mill Road, Cambridge, CB1 2AD',
    postcode: 'CB1 2AD',
    description: 'Erection of two-storey rear extension with associated landscaping',
    appType: 'Full Planning',
    appState: 'Undecided',
    areaName: 'Cambridge City Council',
    startDate: '2026-01-15',
    url: 'https://planit.org.uk/planapplic/APP-001',
    latestUnreadEvent: null,
    ...overrides,
  };
}

export function permittedApplication(
  overrides?: Partial<PlanningApplicationSummary>,
): PlanningApplicationSummary {
  return {
    uid: asApplicationUid('APP-002'),
    name: '2026/0099/LBC',
    address: '45 High Street, Cambridge, CB2 1LA',
    postcode: 'CB2 1LA',
    description: 'Change of use from retail to residential',
    appType: 'Listed Building Consent',
    appState: 'Permitted',
    areaName: 'Cambridge City Council',
    startDate: '2026-02-20',
    url: null,
    latestUnreadEvent: null,
    ...overrides,
  };
}

export function conditionsApplication(
  overrides?: Partial<PlanningApplicationSummary>,
): PlanningApplicationSummary {
  return {
    uid: asApplicationUid('APP-004'),
    name: '2026/0114/FUL',
    address: '7 Trumpington Street, Cambridge, CB2 1QA',
    postcode: 'CB2 1QA',
    description: 'Single-storey side extension with landscaping conditions',
    appType: 'Full Planning',
    appState: 'Conditions',
    areaName: 'Cambridge City Council',
    startDate: '2026-02-25',
    url: null,
    latestUnreadEvent: null,
    ...overrides,
  };
}

export function rejectedApplication(
  overrides?: Partial<PlanningApplicationSummary>,
): PlanningApplicationSummary {
  return {
    uid: asApplicationUid('APP-005'),
    name: '2026/0150/FUL',
    address: '3 King Street, Cambridge, CB1 1LH',
    postcode: 'CB1 1LH',
    description: 'Demolition of garage and erection of two-storey dwelling',
    appType: 'Full Planning',
    appState: 'Rejected',
    areaName: 'Cambridge City Council',
    startDate: '2026-02-28',
    url: null,
    latestUnreadEvent: null,
    ...overrides,
  };
}

export function longDescriptionApplication(
  overrides?: Partial<PlanningApplicationSummary>,
): PlanningApplicationSummary {
  return {
    ...undecidedApplication(),
    uid: asApplicationUid('APP-003'),
    description:
      'Demolition of existing single-storey rear extension and erection of new two-storey rear extension with roof terrace, internal alterations, and associated landscaping works to rear garden',
    ...overrides,
  };
}
