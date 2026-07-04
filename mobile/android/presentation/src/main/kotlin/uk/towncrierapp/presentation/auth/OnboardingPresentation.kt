package uk.towncrierapp.presentation.auth

/**
 * Whether the onboarding wizard should be shown, derived from account state
 * (the user's real watch-zone count) - never a device-local flag alone. Set
 * by [AuthCoordinator] only after ensure-profile and tier resolution have
 * both completed (tc-k9fk/#549: evaluating this any earlier reproduces a
 * real production bug). Port of iOS `OnboardingPresentation`.
 */
public enum class OnboardingPresentation {
    /** Account state hasn't been checked yet this session - render a loading screen, never guess. */
    Undetermined,

    /** The account has zero watch zones. */
    Required,

    /** The account has at least one watch zone. */
    NotRequired,
}
