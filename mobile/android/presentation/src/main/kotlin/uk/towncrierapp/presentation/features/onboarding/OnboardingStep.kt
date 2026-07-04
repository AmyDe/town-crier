package uk.towncrierapp.presentation.features.onboarding

/**
 * The onboarding wizard's four linear steps (tc-7ttz). There is no step
 * before [Welcome] (no back navigation from it) and completion happens from
 * [NotificationPermission] rather than a fifth step.
 */
public enum class OnboardingStep {
    Welcome,
    Postcode,
    Radius,
    NotificationPermission,
}
