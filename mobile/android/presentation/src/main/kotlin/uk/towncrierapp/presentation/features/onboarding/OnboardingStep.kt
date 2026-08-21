package uk.towncrierapp.presentation.features.onboarding

/**
 * The onboarding wizard's steps (tc-7ttz). There is no step before [Welcome]
 * (no back navigation from it) and completion happens from
 * [NotificationPermission] rather than a final step.
 */
public enum class OnboardingStep {
    Welcome,
    Postcode,
    Radius,

    /**
     * Custom-shape boundary drawing (GH#1072 Phase 5, tc-v6fo0.5). Reached
     * ONLY via [OnboardingViewModel.selectCustomShape] (already-entitled
     * re-entry) or [OnboardingViewModel.reconcileTierAfterCustomShapeUpgrade]
     * (a mid-flow purchase completing while sitting on [Radius]) — never
     * part of the default Welcome -> Postcode -> Radius ->
     * NotificationPermission sequence. Port of iOS
     * `OnboardingStep.boundaryDrawing`.
     */
    BoundaryDrawing,
    NotificationPermission,
}
