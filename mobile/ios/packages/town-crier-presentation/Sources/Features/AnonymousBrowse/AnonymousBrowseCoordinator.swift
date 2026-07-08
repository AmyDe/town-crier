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

  /// Fired by "I already have an account", the postcode-entry back button's
  /// sibling CTA paths, and the map's CTA banner / deeper-action taps — all
  /// routes into the existing Auth0 login flow. Wired by the composition root
  /// to `loginViewModel.login()`.
  public var onRequestSignIn: (() -> Void)?

  public init(
    geocoder: PostcodeGeocoder,
    stateRepository: AnonymousBrowseStateRepository,
    applicationsRepository: AnonymousApplicationsRepository
  ) {
    self.geocoder = geocoder
    self.stateRepository = stateRepository
    self.applicationsRepository = applicationsRepository
    self.screen = .welcome
    self.mapViewModel = nil

    // Relaunch persistence (GH#868 Phase 3.5): a saved anonymous session
    // routes straight to the map, never back through welcome.
    if let state = stateRepository.load() {
      screen = .map
      mapViewModel = makeMapViewModel(coordinate: state.coordinate)
    }
  }

  public func makeWelcomeViewModel() -> WelcomeViewModel {
    WelcomeViewModel(
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
      self.mapViewModel = self.makeMapViewModel(coordinate: state.coordinate)
      self.screen = .map
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
    mapViewModel = nil
    screen = .welcome
  }

  private func makeMapViewModel(coordinate: Coordinate) -> AnonymousMapViewModel {
    let viewModel = AnonymousMapViewModel(repository: applicationsRepository, coordinate: coordinate)
    viewModel.onRequestSignUp = { [weak self] in self?.onRequestSignIn?() }
    return viewModel
  }
}
