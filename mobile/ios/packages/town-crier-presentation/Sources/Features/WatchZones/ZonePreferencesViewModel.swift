import Foundation
import TownCrierDomain

/// Drives the per-zone notification preferences screen.
///
/// Loads and saves preferences via ``ZonePreferencesRepository``, with proactive
/// entitlement gating for the status-change and decision-update toggles (Personal+ only),
/// and reactive 403 fallback via ``EntitlementGatingViewModel``.
@MainActor
public final class ZonePreferencesViewModel: ObservableObject, EntitlementGatingViewModel {
  @Published public var newApplications: Bool = true
  @Published public var statusChanges: Bool = false
  @Published public var decisionUpdates: Bool = false
  @Published public private(set) var isLoading = false
  @Published public internal(set) var error: DomainError?
  @Published public var entitlementGate: Entitlement?

  public let featureGate: FeatureGate
  public let zoneName: String

  private let zoneId: String
  private let repository: ZonePreferencesRepository

  public init(
    zoneId: String,
    zoneName: String,
    repository: ZonePreferencesRepository,
    tier: SubscriptionTier
  ) {
    self.zoneId = zoneId
    self.zoneName = zoneName
    self.repository = repository
    self.featureGate = FeatureGate(tier: tier)
  }

  /// Loads the current preferences from the API.
  public func loadPreferences() async {
    isLoading = true
    error = nil

    do {
      let prefs = try await repository.fetchPreferences(zoneId: zoneId)
      newApplications = prefs.newApplications
      statusChanges = prefs.statusChanges
      decisionUpdates = prefs.decisionUpdates
    } catch {
      handleError(error)
    }

    isLoading = false
  }

  /// Saves the current preferences to the API.
  ///
  /// If the API returns 403 insufficient_entitlement, the entitlement gate binding
  /// is set to trigger the subscription upsell sheet.
  public func savePreferences() async {
    error = nil

    let prefs = ZoneNotificationPreferences(
      zoneId: zoneId,
      newApplications: newApplications,
      statusChanges: statusChanges,
      decisionUpdates: decisionUpdates
    )

    do {
      try await repository.updatePreferences(prefs)
    } catch {
      handleError(error)
    }
  }

  /// Proactively triggers the subscription upsell sheet for a gated entitlement.
  ///
  /// Called by ``GatedToggle`` when a Free user taps a disabled toggle.
  public func showUpgradeSheet(for entitlement: Entitlement) {
    self.entitlementGate = entitlement
  }
}
