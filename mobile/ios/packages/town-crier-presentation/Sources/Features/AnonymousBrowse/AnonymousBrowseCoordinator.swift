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
    /// The anonymous tab shell — Applications, Map, Settings (GH#879 Phase
    /// 3; Zones arrives in Phase 4). Replaces the bare map as the
    /// post-postcode destination.
    case tabs
  }

  /// The anonymous tab shell's tabs (GH#879 Phase 3). Deliberately no
  /// `.saved` case — saving is account-bound (ADR 0035) and stays the
  /// conversion pitch; no `.zones` case yet — device-local zones arrive in
  /// Phase 4.
  public enum Tab: Hashable, Sendable {
    case applications
    case map
    case settings
  }

  @Published public private(set) var screen: Screen
  @Published public private(set) var mapViewModel: AnonymousMapViewModel?
  /// Selected tab on the tab shell; bound to its `TabView`.
  @Published public var selectedTab: Tab = .applications

  private let geocoder: PostcodeGeocoder
  private let stateRepository: AnonymousBrowseStateRepository
  private let applicationsRepository: AnonymousApplicationsRepository
  /// The state backing the current map session, kept in sync with the live
  /// radius picker so a slider drag re-persists the postcode/coordinate
  /// alongside the newly chosen radius (GH#868 Phase 3 refinement). Also the
  /// source for the Applications tab's list view model (GH#879 Phase 3).
  private var currentState: AnonymousBrowseState?
  /// Single live source of truth for the appearance preference (GH#878),
  /// shared with `SettingsViewModel` — injected by the composition root so
  /// the welcome screen's appearance control and the root
  /// `.preferredColorScheme` observe the exact same instance.
  private let appearanceStore: AppearanceStore
  /// Backs the anonymous Settings tab's "Version" row (GH#879 Phase 3).
  /// Required (no default): unlike `appearanceStore`, no concrete
  /// implementation lives in this package — `town-crier-presentation`
  /// deliberately does not depend on `town-crier-data` — so the composition
  /// root must inject one, mirroring `AppCoordinator.init`.
  private let appVersionProvider: AppVersionProvider

  /// Fired by "I already have an account", the postcode-entry back button's
  /// sibling CTA paths, and the map's CTA banner / deeper-action taps — all
  /// routes into the existing Auth0 login flow. Wired by the composition root
  /// to `loginViewModel.login()`.
  public var onRequestSignIn: (() -> Void)?

  /// Fired by the anonymous map's "View full details" handoff (GH#879 Phase
  /// 2). Wired by the composition root to present the shared root detail
  /// sheet in anonymous mode (`AppCoordinator.showAnonymousApplicationDetail`).
  public var onShowApplicationDetail: ((PlanningApplication) -> Void)?

  /// Fired by the anonymous Settings tab's "Privacy Policy" row (GH#879
  /// Phase 3). Wired by the composition root to the shared
  /// `AppCoordinator.showPrivacyPolicy()` — legal documents are loaded from a
  /// bundled JSON resource with no network/auth dependency, so both the
  /// authed and anonymous surfaces can safely reuse the same presentation
  /// mechanism on the always-present `AppCoordinator` instance.
  public var onShowPrivacyPolicy: (() -> Void)?
  /// Fired by the anonymous Settings tab's "Terms of Service" row (GH#879
  /// Phase 3). See ``onShowPrivacyPolicy``.
  public var onShowTermsOfService: (() -> Void)?
  /// Fired by the anonymous Settings tab's "Rate the App" row (GH#879 Phase
  /// 3). Wired by the composition root to the shared
  /// `AppCoordinator.rateApp()`.
  public var onRateApp: (() -> Void)?

  public init(
    geocoder: PostcodeGeocoder,
    stateRepository: AnonymousBrowseStateRepository,
    applicationsRepository: AnonymousApplicationsRepository,
    appearanceStore: AppearanceStore? = nil,
    appVersionProvider: AppVersionProvider
  ) {
    self.geocoder = geocoder
    self.stateRepository = stateRepository
    self.applicationsRepository = applicationsRepository
    self.appearanceStore = appearanceStore ?? AppearanceStore()
    self.appVersionProvider = appVersionProvider
    self.screen = .welcome
    self.mapViewModel = nil

    // Relaunch persistence (GH#868 Phase 3.5): a saved anonymous session
    // routes straight to the tab shell, never back through welcome.
    if let state = stateRepository.load() {
      currentState = state
      screen = .tabs
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
      screen = .tabs
    }
    return viewModel
  }

  /// Returns the flow to its zero state and clears any persisted anonymous
  /// session — called on sign-out (GH#868 Phase 3.6), a deliberate return to
  /// the welcome screen rather than back to the anonymous tab shell.
  /// Idempotent: safe to call even when nothing was ever persisted (e.g. a
  /// user who was never anonymous before signing in).
  public func reset() {
    stateRepository.clear()
    currentState = nil
    mapViewModel = nil
    screen = .welcome
  }

  // MARK: - Tab shell factories (GH#879 Phase 3)

  /// Builds the Applications tab's list view model, sourced from the same
  /// coordinate/radius the map preview uses. Returns `nil` only in the
  /// practically-unreachable case the tab shell renders before postcode
  /// resolution has completed — `screen` only transitions to `.tabs` once
  /// `currentState` (and `mapViewModel`) are both set.
  public func makeApplicationListViewModel() -> AnonymousApplicationListViewModel? {
    guard let state = currentState else { return nil }
    let viewModel = AnonymousApplicationListViewModel(
      repository: applicationsRepository,
      coordinate: state.coordinate,
      radiusMetres: state.radiusMetres)
    viewModel.onShowApplicationDetail = { [weak self] application in
      self?.onShowApplicationDetail?(application)
    }
    return viewModel
  }

  /// Builds the Settings tab's view model, sharing the same live
  /// ``AppearanceStore`` instance the welcome screen and (once signed in)
  /// `SettingsViewModel` observe.
  public func makeSettingsViewModel() -> AnonymousSettingsViewModel {
    AnonymousSettingsViewModel(
      appearanceStore: appearanceStore, appVersionProvider: appVersionProvider)
  }

  /// "Create free account" / "Sign in" — both routes into the same Auth0
  /// entry point every sign-up surface in the app uses. Called directly by
  /// `AnonymousMainTabView`'s CTA banner and by `AnonymousSettingsView`'s
  /// create-account section (mirrors `MainTabView` calling
  /// `coordinator.showSettings()` directly).
  public func requestSignIn() {
    onRequestSignIn?()
  }

  /// "Privacy Policy" row on the anonymous Settings tab.
  public func showPrivacyPolicy() {
    onShowPrivacyPolicy?()
  }

  /// "Terms of Service" row on the anonymous Settings tab.
  public func showTermsOfService() {
    onShowTermsOfService?()
  }

  /// "Rate the App" row on the anonymous Settings tab.
  public func requestRateApp() {
    onRateApp?()
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
