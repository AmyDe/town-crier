import type { SavedApplication, PlanningApplicationSummary } from '../../../../domain/types';
import { asApplicationUid } from '../../../../domain/types';

function summaryForUid(uid: string, overrides?: Partial<PlanningApplicationSummary>): PlanningApplicationSummary {
  return {
    uid: asApplicationUid(uid),
    name: '2026/0042/FUL',
    address: '12 Mill Road, Cambridge, CB1 2AD',
    postcode: 'CB1 2AD',
    description: 'Erection of two-storey rear extension',
    appType: 'Full Planning',
    appState: 'Undecided',
    areaName: 'Cambridge City Council',
    startDate: '2026-01-15',
    url: 'https://planit.org.uk/planapplic/' + uid,
    ...overrides,
  };
}

export function savedUndecidedApplication(
  overrides?: Partial<SavedApplication>,
): SavedApplication {
  const uid = overrides?.applicationUid ?? asApplicationUid('APP-001');
  return {
    applicationUid: uid,
    savedAt: '2026-03-01T10:00:00Z',
    application: summaryForUid(uid, overrides?.application),
    ...overrides,
  };
}

export function savedPermittedApplication(
  overrides?: Partial<SavedApplication>,
): SavedApplication {
  const uid = overrides?.applicationUid ?? asApplicationUid('APP-002');
  return {
    applicationUid: uid,
    savedAt: '2026-03-05T14:30:00Z',
    application: summaryForUid(uid, {
      name: '2026/0099/LBC',
      address: '45 High Street, Cambridge, CB2 1LA',
      postcode: 'CB2 1LA',
      description: 'Change of use from retail to residential',
      appType: 'Listed Building Consent',
      appState: 'Permitted',
      startDate: '2026-02-20',
      url: null,
      ...overrides?.application,
    }),
    ...overrides,
  };
}
