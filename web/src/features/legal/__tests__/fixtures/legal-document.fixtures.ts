import type { LegalDocument } from '../../../../domain/types';

export function privacyPolicy(
  overrides?: Partial<LegalDocument>,
): LegalDocument {
  return {
    documentType: 'privacy',
    title: 'Privacy Policy',
    lastUpdated: '2026-03-16',
    sections: [
      { heading: 'What We Collect', body: 'We collect minimal data.' },
      { heading: 'Your Rights', body: 'You have the right to access your data.' },
    ],
    ...overrides,
  };
}

export function termsOfService(
  overrides?: Partial<LegalDocument>,
): LegalDocument {
  return {
    documentType: 'terms',
    title: 'Terms of Service',
    lastUpdated: '2026-03-16',
    sections: [
      { heading: 'Acceptance of Terms', body: 'By using Town Crier, you agree.' },
      { heading: 'Service Description', body: 'Town Crier provides notifications.' },
    ],
    ...overrides,
  };
}
