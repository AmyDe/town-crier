package uk.towncrierapp.domain.onboarding

/**
 * Device-local "has this device been through onboarding" latch. This is a
 * fast-path hint only, for perceived speed on a returning user's device -
 * account state (the user's real watch-zone count, checked by
 * `AuthCoordinator`) is always the source of truth for whether the wizard is
 * required, and never gets substituted by this latch. Port of iOS
 * `OnboardingRepository` / `UserDefaultsOnboardingRepository`.
 */
public interface OnboardingRepository {
    /** Whether this device has completed onboarding before. */
    public suspend fun isOnboardingComplete(): Boolean

    public suspend fun setOnboardingComplete(complete: Boolean)
}
