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
    /// The anonymous tab shell — Applications, Map, Zones, Settings.
    /// Replaces the bare map as the post-postcode destination.
    case tabs
  }

  /// The anonymous tab shell's tabs, in display order (GH#879 Phase 3;
  /// `.zones` added Phase 4). Deliberately no `.saved` case — saving is
  /// account-bound (ADR 0035) and stays the conversion pitch. `CaseIterable`
  /// so tests can assert the tab set is exactly these four, with no silent
  /// additions.
  public enum Tab: Hashable, CaseIterable, Sendable {
    case applications
    case map
    case zones
    case settings
  }

  @Published public private(set) var screen: Screen
  @Published public private(set) var mapViewModel: AnonymousMapViewModel?
  /// Selected tab on the tab shell; bound to its `TabView`.
  @Published public var selectedTab: Tab = .applications

  private let geocoder: PostcodeGeocoder
  private let stateRepository: AnonymousBrowseStateRepository
  private let applicationsRepository: AnonymousApplicationsRepository
  /// Backs the Zones tab (GH#879 Phase 4) and the Applications tab's
  /// zone-driven query/picker. A distinct store from `stateRepository` — see
  /// `DeviceLocalZoneRepository`'s own docs for the migration relationship
  /// between the two.
  private let deviceLocalZoneRepository: DeviceLocalZoneRepository
  /// The state backing the current map session, kept in sync with the live
  /// radius picker so a slider drag re-persists the postcode/coordinate
  /// alongside the newly chosen radius (GH#868 Phase 3 refinement). Also the
  /// source for the Applications tab's list view model (GH#879 Phase 3).
  private var currentState: AnonymousBrowseState?
  /// The live Applications list view model, cached the first time
  /// ``makeApplicationListViewModel()`` builds one, and returned unchanged on
  /// every later call (GH#888). Necessary for the SAME reason
  /// ``mapViewModel`` is a stored property mutated in place rather than
  /// rebuilt: `AnonymousMainTabView`'s tab content is re-evaluated on every
  /// `selectedTab`/coordinator change (`TabView` builds every tab's content
  /// closure up front, not just the selected one), so a naive "build fresh
  /// every call" factory would hand back a throwaway instance the mounted
  /// `AnonymousApplicationListView`'s `@StateObject` immediately discards —
  /// leaving nothing for a later Zones-tab edit to refetch. Caching the
  /// first instance gives ``makeDeviceLocalZoneListViewModel()``'s
  /// `onZonesChanged` wiring a stable handle to call `loadApplications()` on.
  private var applicationListViewModel: AnonymousApplicationListViewModel?
  /// Test-only synchronisation handle for the Applications-list refetch
  /// ``makeDeviceLocalZoneListViewModel()`` kicks off after a Zones-tab edit
  /// — mirrors `AnonymousMapViewModel.waitForPendingRegionChangeRefetch()`.
  private var pendingZoneEditRefetchTask: Task<Void, Never>?
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
    deviceLocalZoneRepository: DeviceLocalZoneRepository,
    appearanceStore: AppearanceStore? = nil,
    appVersionProvider: AppVersionProvider
  ) {
    self.geocoder = geocoder
    self.stateRepository = stateRepository
    self.applicationsRepository = applicationsRepository
    self.deviceLocalZoneRepository = deviceLocalZoneRepository
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
    applicationListViewModel = nil
    screen = .welcome
  }

  // MARK: - Tab shell factories (GH#879 Phase 3)

  /// Builds the Applications tab's list view model. The active device-local
  /// zone drives the query (GH#879 Phase 4); `state.coordinate`/
  /// `state.radiusMetres` back it only as a fallback for the
  /// practically-unreachable case no device-local zone exists at all.
  /// Returns `nil` only in the practically-unreachable case the tab shell
  /// renders before postcode resolution has completed — `screen` only
  /// transitions to `.tabs` once `currentState` (and `mapViewModel`) are both
  /// set.
  public func makeApplicationListViewModel() -> AnonymousApplicationListViewModel? {
    // Cached (GH#888) — see `applicationListViewModel`'s own docs for why a
    // fresh instance on every call would be silently discarded by the
    // mounted view's `@StateObject`, leaving nothing for a Zones-tab edit to
    // refetch later.
    if let applicationListViewModel { return applicationListViewModel }
    guard let state = currentState else { return nil }
    let viewModel = AnonymousApplicationListViewModel(
      repository: applicationsRepository,
      zoneRepository: deviceLocalZoneRepository,
      fallbackCoordinate: state.coordinate,
      fallbackRadiusMetres: state.radiusMetres)
    viewModel.onShowApplicationDetail = { [weak self] application in
      self?.onShowApplicationDetail?(application)
    }
    // Switching the active zone (a picker chip tap) re-centres the Map tab
    // to match (GH#879 Phase 4 acceptance criteria). Mutates the EXISTING
    // `mapViewModel` in place via `updateActiveZone(_:)` rather than
    // replacing the published property with a new instance — live simulator
    // verification found that replacing it left the Map tab frozen on the
    // previous zone until a full relaunch, because `AnonymousMapView` holds
    // the view model in a `@StateObject`, which SwiftUI keeps bound to
    // whichever instance was FIRST passed to it (see
    // `AnonymousMapViewModel.updateActiveZone(_:)`'s own docs for the full
    // writeup).
    viewModel.onActiveZoneChanged = { [weak self] zone in
      self?.mapViewModel?.updateActiveZone(zone)
    }
    // The "Add area" chip's sign-up CTA (GH#888) — the on-device cap is one
    // zone, so adding another always routes here.
    viewModel.onRequestSignUp = { [weak self] in self?.onRequestSignIn?() }
    applicationListViewModel = viewModel
    return viewModel
  }

  /// Builds the Zones tab's view model, sharing the same anonymous
  /// `PostcodeGeocoder` instance postcode entry uses (never `/v1/geocode`).
  ///
  /// GH#888: a successful edit fires ``DeviceLocalZoneListViewModel/onZonesChanged``
  /// with the saved zone. That mutates the EXISTING `mapViewModel` in place
  /// via `updateActiveZone(_:)` — the same identity-preserving requirement
  /// `onActiveZoneChanged` above satisfies, and for the same reason (see
  /// that wiring's docs) — and kicks off a refetch of the cached
  /// ``applicationListViewModel`` so the Applications tab picks up the edit
  /// without a relaunch.
  public func makeDeviceLocalZoneListViewModel() -> DeviceLocalZoneListViewModel {
    let viewModel = DeviceLocalZoneListViewModel(
      repository: deviceLocalZoneRepository, geocoder: geocoder)
    viewModel.onRequestSignUp = { [weak self] in self?.onRequestSignIn?() }
    viewModel.onZonesChanged = { [weak self] zone in
      guard let self else { return }
      mapViewModel?.updateActiveZone(zone)
      pendingZoneEditRefetchTask = Task { [weak self] in
        await self?.applicationListViewModel?.loadApplications()
      }
    }
    return viewModel
  }

  /// Test-only synchronisation: await the most recently scheduled
  /// Applications-list refetch triggered by a Zones-tab edit (GH#888),
  /// mirroring `AnonymousMapViewModel.waitForPendingRegionChangeRefetch()`.
  public func waitForPendingZoneEditRefetch() async {
    await pendingZoneEditRefetchTask?.value
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
