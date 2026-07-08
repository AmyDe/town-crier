import TownCrierDomain

/// Whether the first-run onboarding wizard should be presented for the
/// authenticated user.
///
/// The gate is account-state driven (does the user have zero watch zones?)
/// rather than a device-local latch, so it survives reinstall and works across
/// devices. ``AppCoordinator/determineOnboarding()`` resolves this after
/// profile-ensure completes; the third `undetermined` case keeps the wizard
/// from flashing while watch zones are still loading (tc-w3cb.1).
public enum OnboardingPresentation: Equatable, Sendable {
  /// Still resolving — show a neutral loading screen, never the wizard.
  case undetermined
  /// The user has no watch zones — guide them through creating their first.
  case required
  /// The user already has at least one zone — go straight to the app.
  case notRequired
}

extension AppCoordinator {
  /// Factory for the first-run onboarding wizard. Mirrors
  /// ``makeWatchZoneEditorViewModel(editing:)`` in guarding the optional
  /// `geocoder`, and returns a retained instance so the live subscription tier
  /// can be pushed into it (see ``resolveSubscriptionTier()``).
  public func makeOnboardingViewModel() -> OnboardingViewModel {
    guard let geocoder else {
      fatalError("PostcodeGeocoder must be injected to create OnboardingViewModel")
    }
    if let cached = onboardingViewModel {
      // Re-renders re-evaluate this factory; keep the tier current but reuse
      // the live instance so wizard state (postcode, geocode) is preserved.
      cached.subscriptionTier = subscriptionTier
      return cached
    }
    let viewModel = OnboardingViewModel(
      geocoder: geocoder,
      watchZoneRepository: watchZoneRepository,
      onboardingRepository: onboardingRepository,
      notificationService: notificationService,
      subscriptionTier: subscriptionTier
    )
    viewModel.onComplete = { [weak self] _ in
      self?.completeOnboarding()
    }
    // In-wizard radius upsell (tc-w3cb.3): build the paywall and, on dismiss,
    // re-resolve the tier so the larger radius unlocks live. resolveSubscriptionTier
    // pushes the new tier back into this same VM instance.
    viewModel.makeUpsellViewModel = { [weak self] in
      self?.makeSubscriptionViewModel()
    }
    viewModel.onUpgradeFlowCompleted = { [weak self] in
      await self?.resolveSubscriptionTier()
    }

    // Anonymous browse post-signup handoff (GH#868 Phase 3.5): a user who
    // located themselves before creating an account carries that postcode/
    // coordinate straight into the wizard, landing on the radius step instead
    // of being asked again. Only applies on fresh construction (never the
    // cached-instance return above), so it fires at most once per session.
    // Anonymous state is cleared immediately after so a future sign-out
    // starts from the welcome screen, not a stale anonymous map.
    if let anonymousBrowseStateRepository, let state = anonymousBrowseStateRepository.load() {
      viewModel.prefill(postcode: state.postcode, coordinate: state.coordinate)
      anonymousBrowseStateRepository.clear()
    }

    onboardingViewModel = viewModel
    return viewModel
  }

  /// Resolves whether to show the onboarding wizard for the authenticated user.
  ///
  /// MUST run after profile-ensure (`POST /v1/me`, performed by
  /// ``resolveSubscriptionTier()``) so the wizard's first watch-zone save has a
  /// server profile to attach to — otherwise that save 500s on its quota check
  /// (the GA bug fixed in tc-k9fk).
  ///
  /// The gate is the user's watch-zone count: zero zones means show the wizard.
  /// We only drop back to `.undetermined` (the loading screen) when the device
  /// has no completion latch, so returning users never see a launch flash; a
  /// failed load falls through to the app rather than trapping the user.
  public func determineOnboarding() async {
    if !onboardingRepository.isOnboardingComplete {
      onboardingPresentation = .undetermined
    }
    do {
      let zones = try await watchZoneRepository.loadAll()
      onboardingPresentation = zones.isEmpty ? .required : .notRequired
      // Never ask for a review during a first-run onboarding session (GH #628).
      if zones.isEmpty {
        reviewPromptTracker?.suppressThisSession()
      }
    } catch {
      onboardingPresentation = .notRequired
    }
  }

  /// Invoked when the wizard saves the user's first zone. Flips the gate so the
  /// root view falls through to the main app and releases the wizard VM.
  func completeOnboarding() {
    onboardingPresentation = .notRequired
    onboardingViewModel = nil
  }
}
