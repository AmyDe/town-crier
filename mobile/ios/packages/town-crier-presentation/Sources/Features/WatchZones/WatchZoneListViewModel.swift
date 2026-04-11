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

  /// The proactive feature gate derived from the user's subscription tier.
  public let featureGate: FeatureGate

  var onAddZone: (() -> Void)?
  var onEditZone: ((WatchZone) -> Void)?
  var onUpgradeRequired: (() -> Void)?

  private let repository: WatchZoneRepository

  public init(
    repository: WatchZoneRepository,
    featureGate: FeatureGate
  ) {
    self.repository = repository
    self.featureGate = featureGate
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

  public func load() async {
    isLoading = true
    error = nil

    do {
      zones = try await repository.loadAll()
    } catch {
      handleError(error)
    }

    isLoading = false
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
      onUpgradeRequired?()
    }
  }

  public func editZone(_ zone: WatchZone) {
    onEditZone?(zone)
  }
}
