import Foundation
import TownCrierDomain

/// Owns navigation for the anonymous (pre-signup) browse flow (GH#868 Phase
/// 3): welcome -> postcode entry -> map, plus the handoff into the existing
/// Auth0 flow from any of the three screens. Mirrors the extension-per-concern
/// composition pattern `AppCoordinator+Onboarding.swift` establishes, kept as
/// its own type (rather than an `AppCoordinator` extension) because this flow
/// runs entirely before authentication — `AppCoordinator` represents the
/// authenticated app shell.
///
/// Persisted anonymous state (postcode + coordinate) only exists once postcode
/// entry succeeds; until then the flow lives entirely in ``screen``, mirroring
/// how the onboarding wizard's `OnboardingViewModel.currentStep` stays
/// in-memory until its own completion writes through.
@MainActor
public final class AnonymousBrowseCoordinator: ObservableObject {
  public enum Screen: Equatable, Sendable {
    case welcome
    case postcodeEntry
    case map
  }

  @Published public private(set) var screen: Screen
  @Published public private(set) var mapViewModel: AnonymousMapViewModel?

  private let geocoder: PostcodeGeocoder
  private let stateRepository: AnonymousBrowseStateRepository
  private let applicationsRepository: AnonymousApplicationsRepository
  /// The state backing the current map session, kept in sync with the live
  /// radius picker so a slider drag re-persists the postcode/coordinate
  /// alongside the newly chosen radius (GH#868 Phase 3 refinement).
  private var currentState: AnonymousBrowseState?
  /// Single live source of truth for the appearance preference (GH#878),
  /// shared with `SettingsViewModel` — injected by the composition root so
  /// the welcome screen's appearance control and the root
  /// `.preferredColorScheme` observe the exact same instance.
  private let appearanceStore: AppearanceStore

  /// Fired by "I already have an account", the postcode-entry back button's
  /// sibling CTA paths, and the map's CTA banner / deeper-action taps — all
  /// routes into the existing Auth0 login flow. Wired by the composition root
  /// to `loginViewModel.login()`.
  public var onRequestSignIn: (() -> Void)?

  /// Fired by the anonymous map's "View full details" handoff (GH#879 Phase
  /// 2). Wired by the composition root to present the shared root detail
  /// sheet in anonymous mode (`AppCoordinator.showAnonymousApplicationDetail`).
  public var onShowApplicationDetail: ((PlanningApplication) -> Void)?

  public init(
    geocoder: PostcodeGeocoder,
    stateRepository: AnonymousBrowseStateRepository,
    applicationsRepository: AnonymousApplicationsRepository,
    appearanceStore: AppearanceStore? = nil
  ) {
    self.geocoder = geocoder
    self.stateRepository = stateRepository
    self.applicationsRepository = applicationsRepository
    self.appearanceStore = appearanceStore ?? AppearanceStore()
    self.screen = .welcome
    self.mapViewModel = nil

    // Relaunch persistence (GH#868 Phase 3.5): a saved anonymous session
    // routes straight to the map, never back through welcome.
    if let state = stateRepository.load() {
      currentState = state
      screen = .map
      mapViewModel = makeMapViewModel(state: state)
    }
  }

  public func makeWelcomeViewModel() -> WelcomeViewModel {
    WelcomeViewModel(
      appearanceStore: appearanceStore,
      onGetStarted: { [weak self] in self?.screen = .postcodeEntry },
      onSignIn: { [weak self] in self?.onRequestSignIn?() }
    )
  }

  public func makePostcodeEntryViewModel() -> AnonymousPostcodeEntryViewModel {
    let viewModel = AnonymousPostcodeEntryViewModel(
      geocoder: geocoder, stateRepository: stateRepository)
    viewModel.onBack = { [weak self] in self?.screen = .welcome }
    viewModel.onResolved = { [weak self] state in
      guard let self else { return }
      currentState = state
      mapViewModel = makeMapViewModel(state: state)
      screen = .map
    }
    return viewModel
  }

  /// Returns the flow to its zero state and clears any persisted anonymous
  /// session — called on sign-out (GH#868 Phase 3.6), a deliberate return to
  /// the welcome screen rather than back to the anonymous map. Idempotent:
  /// safe to call even when nothing was ever persisted (e.g. a user who was
  /// never anonymous before signing in).
  public func reset() {
    stateRepository.clear()
    currentState = nil
    mapViewModel = nil
    screen = .welcome
  }

  private func makeMapViewModel(state: AnonymousBrowseState) -> AnonymousMapViewModel {
    let viewModel = AnonymousMapViewModel(
      repository: applicationsRepository,
      coordinate: state.coordinate,
      radiusMetres: state.radiusMetres)
    viewModel.onRequestSignUp = { [weak self] in self?.onRequestSignIn?() }
    viewModel.onRadiusChanged = { [weak self] radius in self?.persistRadius(radius) }
    viewModel.onShowApplicationDetail = { [weak self] application in
      self?.onShowApplicationDetail?(application)
    }
    return viewModel
  }

  /// Re-saves the current session's postcode/coordinate with a newly chosen
  /// radius (GH#868 Phase 3 refinement) so the post-signup handoff into
  /// onboarding carries whatever the user last picked. No-op if called
  /// before any state exists (shouldn't happen: the radius picker only shows
  /// once the map — and therefore `currentState` — exists).
  private func persistRadius(_ radiusMetres: Double) {
    guard let state = currentState else { return }
    let updated = AnonymousBrowseState(
      postcode: state.postcode,
      coordinate: state.coordinate,
      radiusMetres: radiusMetres,
      createdAt: state.createdAt)
    currentState = updated
    stateRepository.save(updated)
  }
}
