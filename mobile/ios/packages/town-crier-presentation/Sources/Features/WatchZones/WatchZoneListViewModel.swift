import Combine
import Foundation
import TownCrierDomain

/// Manages the list of user's watch zones with proactive tier-based gating.
///
/// The ``FeatureGate`` is injected at construction time from the session's
/// subscription tier, enabling quota checks and upgrade badge logic without
/// a network round-trip.
@MainActor
public final class WatchZoneListViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published public private(set) var zones: [WatchZone] = []
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  @Published public var isUpgradePromptPresented = false
  /// Device-local zones (GH#879 Phase 4) not yet converted to a real
  /// `WatchZone` — populated by ``load()`` from the injected
  /// `deviceLocalZoneRepository`. Always empty when no repository was
  /// injected (existing call sites/tests are unaffected).
  @Published public private(set) var unconvertedLocalZones: [DeviceLocalZone] = []
  /// Session-only dismissal of the unconverted-zones row (GH#879 Phase 5).
  /// Never persisted — a fresh `WatchZoneListViewModel` (next app launch)
  /// always starts un-dismissed, so the row reappears while zones remain.
  @Published public private(set) var isLocalZoneRowDismissed = false
  /// Drives the row's delete-confirmation dialog (tc-luq4u) — set by
  /// ``presentDiscardConfirmation()`` when the user taps the row's "x", and
  /// cleared by whichever choice they make (``discardLocalZones()`` or
  /// ``dismissLocalZoneRow()``).
  @Published public var isDiscardConfirmationPresented = false

  /// The proactive feature gate derived from the user's subscription tier.
  public let featureGate: FeatureGate

  var onAddZone: (() -> Void)?
  var onEditZone: ((WatchZone) -> Void)?
  var onUpgradeRequired: (() -> Void)?
  var onViewPlans: (() -> Void)?
  /// Fired when the user taps the unconverted-zones row (GH#879 Phase 5) —
  /// wired by the coordinator to reopen the conversion sheet.
  var onConvertLocalZones: (() -> Void)?

  private let repository: WatchZoneRepository
  private let deviceLocalZoneRepository: DeviceLocalZoneRepository?

  public init(
    repository: WatchZoneRepository,
    featureGate: FeatureGate,
    deviceLocalZoneRepository: DeviceLocalZoneRepository? = nil
  ) {
    self.repository = repository
    self.featureGate = featureGate
    self.deviceLocalZoneRepository = deviceLocalZoneRepository
  }

  /// True while any unconverted device-local zone remains AND the user has
  /// not dismissed the row this session — clears entirely once none remain,
  /// independent of the dismiss flag (never keep an empty row dismissable).
  public var showsLocalZoneRow: Bool {
    !unconvertedLocalZones.isEmpty && !isLocalZoneRowDismissed
  }

  /// Hides the unconverted-zones row for the rest of this session. The row
  /// reappears on next launch (a fresh view model) while zones still remain
  /// — local zones are never silently discarded, only converted or
  /// explicitly deleted. This is the "keep for later" choice in the
  /// delete-confirmation dialog (tc-luq4u).
  public func dismissLocalZoneRow() {
    isLocalZoneRowDismissed = true
    isDiscardConfirmationPresented = false
  }

  /// Opens the row's delete-confirmation dialog (tc-luq4u) — the "x" button
  /// no longer dismisses directly, since dismissal alone gave a pre-signup
  /// user with no onboarding-wizard run no way to ever decline the zones the
  /// row nags about.
  public func presentDiscardConfirmation() {
    isDiscardConfirmationPresented = true
  }

  /// Permanently deletes every unconverted device-local zone (tc-luq4u) —
  /// the explicit "no" the row previously lacked. Deletes each from the
  /// injected repository, then clears the published list so
  /// ``showsLocalZoneRow`` becomes false immediately and stays false on
  /// every later ``load()`` (the repository itself is now empty).
  public func discardLocalZones() {
    for zone in unconvertedLocalZones {
      deviceLocalZoneRepository?.delete(zone.id)
    }
    unconvertedLocalZones = []
    isDiscardConfirmationPresented = false
  }

  /// Reopens the post-signup conversion sheet from the row's tap target.
  public func convertLocalZones() {
    onConvertLocalZones?()
  }

  /// Whether the user can add another watch zone given their tier and current count.
  public var canAddZone: Bool {
    featureGate.canAdd(quota: .watchZones, currentCount: zones.count)
  }

  /// Whether the "Upgrade" badge should be shown on the add-zone button.
  ///
  /// True when the user has reached their tier's zone limit.
  public var showUpgradeBadge: Bool {
    featureGate.shouldShowUpgradeBadge(for: .watchZones, currentCount: zones.count)
  }

  /// Whether to show the richer free-tier inline upsell card beneath the zone list.
  ///
  /// Single source of truth for that card. True only for a free-tier user who has
  /// used their single allowed zone. Paid users never see it, including a Personal
  /// user sitting at their finite 3-zone cap (where `showUpgradeBadge` is also true,
  /// which is why the card must not piggyback on it). Below-cap free users see
  /// nothing. Because tier-keyed views rebuild on `coordinator.subscriptionTier`,
  /// the card disappears live after an in-app purchase with no extra work.
  public var showsFreeTierUpsell: Bool {
    featureGate.tier == .free && !canAddZone
  }

  /// Local-vs-server centre match tolerance for duplicate cleanup (tc-luq4u)
  /// — 1e-4 degrees is well under one metre at UK latitudes, generous enough
  /// to absorb floating-point drift from a conversion round-trip while never
  /// matching two genuinely distinct areas.
  private static let duplicateCentreToleranceDegrees: Double = 1e-4

  public func load() async {
    isLoading = true
    error = nil

    var serverLoadSucceeded = true
    do {
      zones = try await repository.loadAll()
    } catch {
      handleError(error)
      serverLoadSucceeded = false
    }

    let localZones = deviceLocalZoneRepository?.loadAll() ?? []
    unconvertedLocalZones =
      serverLoadSucceeded
      ? localZones.filter { !discardIfDuplicatesServerZone($0) }
      : localZones

    isLoading = false
  }

  /// Deletes `localZone` from the repository and returns true when it
  /// duplicates an already-converted server zone (same radius, centre
  /// within ``duplicateCentreToleranceDegrees`` on both latitude and
  /// longitude) — a redundant leftover from a conversion that saved
  /// server-side but never cleaned up its local copy. Only ever called
  /// after a successful server load (tc-luq4u): a local zone is never
  /// deleted against an unknown server state.
  private func discardIfDuplicatesServerZone(_ localZone: DeviceLocalZone) -> Bool {
    let isDuplicate = zones.contains { serverZone in
      serverZone.radiusMetres == localZone.radiusMetres
        && abs(serverZone.centre.latitude - localZone.centre.latitude)
          <= Self.duplicateCentreToleranceDegrees
        && abs(serverZone.centre.longitude - localZone.centre.longitude)
          <= Self.duplicateCentreToleranceDegrees
    }
    if isDuplicate {
      deviceLocalZoneRepository?.delete(localZone.id)
    }
    return isDuplicate
  }

  public func deleteZone(_ zone: WatchZone) async {
    error = nil
    do {
      try await repository.delete(zone.id)
      zones.removeAll { $0.id == zone.id }
    } catch {
      handleError(error)
    }
  }

  public func addZone() {
    if canAddZone {
      onAddZone?()
    } else {
      isUpgradePromptPresented = true
      onUpgradeRequired?()
    }
  }

  /// Dismisses the upgrade prompt without navigating to subscription plans.
  public func dismissUpgradePrompt() {
    isUpgradePromptPresented = false
  }

  /// Navigates to subscription plans and dismisses the upgrade prompt.
  public func viewPlans() {
    isUpgradePromptPresented = false
    onViewPlans?()
  }

  /// Value proposition text shown in the upsell prompt.
  public var upgradeValueProposition: String {
    "Monitor multiple areas at once. Upgrade to add more watch zones and never miss a planning application near you."
  }

  public func editZone(_ zone: WatchZone) {
    onEditZone?(zone)
  }
}
