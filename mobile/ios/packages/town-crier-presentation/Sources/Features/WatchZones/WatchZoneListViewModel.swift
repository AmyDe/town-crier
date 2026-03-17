import Combine
import Foundation
import TownCrierDomain

/// Manages the list of user's watch zones with tier-based limits.
@MainActor
public final class WatchZoneListViewModel: ObservableObject {
    @Published public private(set) var zones: [WatchZone] = []
    @Published public private(set) var isLoading = false
    @Published public private(set) var error: DomainError?
    @Published public private(set) var currentTier: SubscriptionTier = .free

    var onAddZone: (() -> Void)?
    var onEditZone: ((WatchZone) -> Void)?
    var onUpgradeRequired: (() -> Void)?

    private let repository: WatchZoneRepository
    private let subscriptionService: SubscriptionService

    public init(
        repository: WatchZoneRepository,
        subscriptionService: SubscriptionService
    ) {
        self.repository = repository
        self.subscriptionService = subscriptionService
    }

    public var canAddZone: Bool {
        let limits = WatchZoneLimits(tier: currentTier)
        return limits.canAddZone(currentCount: zones.count)
    }

    public func load() async {
        isLoading = true
        error = nil

        if let entitlement = await subscriptionService.currentEntitlement() {
            currentTier = entitlement.tier
        } else {
            currentTier = .free
        }

        do {
            zones = try await repository.loadAll()
        } catch let domainError as DomainError {
            error = domainError
        } catch {
            self.error = .unexpected(error.localizedDescription)
        }

        isLoading = false
    }

    public func deleteZone(_ zone: WatchZone) async {
        error = nil
        do {
            try await repository.delete(zone.id)
            zones.removeAll { $0.id == zone.id }
        } catch let domainError as DomainError {
            error = domainError
        } catch {
            self.error = .unexpected(error.localizedDescription)
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
