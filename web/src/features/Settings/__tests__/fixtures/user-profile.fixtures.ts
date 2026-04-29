import type { UserProfile, SubscriptionTier } from '../../../../domain/types';

export function freeUserProfile(
  overrides?: Partial<UserProfile>,
): UserProfile {
  return {
    userId: 'auth0|abc123',
    pushEnabled: true,
    emailDigestEnabled: true,
    digestDay: 1,
    tier: 'Free' as SubscriptionTier,
    ...overrides,
  };
}

export function proUserProfile(
  overrides?: Partial<UserProfile>,
): UserProfile {
  return {
    ...freeUserProfile(),
    userId: 'auth0|pro456',
    tier: 'Pro' as SubscriptionTier,
    ...overrides,
  };
}
