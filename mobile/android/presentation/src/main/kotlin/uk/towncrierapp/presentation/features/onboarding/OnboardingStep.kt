package uk.towncrierapp.presentation.features.onboarding

/**
 * The onboarding wizard's steps. There is no step before [Welcome]
 * (no back navigation from it) and completion happens from
 * [NotificationPermission] rather than a final step.
 */
public enum class OnboardingStep {
    Welcome,
    Postcode,
    Radius,

    /**
     * Custom-shape boundary drawing. Reached
     * ONLY via [OnboardingViewModel.selectCustomShape] (already-entitled
     * re-entry) or [OnboardingViewModel.reconcileTierAfterCustomShapeUpgrade]
     * (a mid-flow purchase completing while sitting on [Radius]) — never
     * part of the default Welcome -> Postcode -> Radius ->
     * NotificationPermission sequence.
     */
    BoundaryDrawing,
    NotificationPermission,
}
