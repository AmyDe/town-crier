import Foundation
import TownCrierDomain

/// ViewModel driving the dashboard -- the post-onboarding landing screen.
///
/// Combines watch zone summary, application authorities, and navigation
/// quick links. Available to all tiers with no entitlement gating.
@MainActor
public final class DashboardViewModel: ObservableObject, ErrorHandlingViewModel {
  @Published private(set) var zones: [WatchZone] = []
  @Published private(set) var authorities: [LocalAuthority] = []
  @Published private(set) var isLoading = false
  @Published var error: DomainError?

  /// The proactive feature gate derived from the user's subscription tier.
  public let featureGate: FeatureGate

  // MARK: - Navigation callbacks

  var onNavigateToZones: (() -> Void)?
  var onNavigateToSaved: (() -> Void)?
  var onNavigateToNotifications: (() -> Void)?
  var onNavigateToMap: (() -> Void)?
  var onNavigateToAuthority: ((LocalAuthority) -> Void)?

  private let watchZoneRepository: WatchZoneRepository
  private let authorityRepository: ApplicationAuthorityRepository
  private var authorityResult: ApplicationAuthorityResult?

  public init(
    watchZoneRepository: WatchZoneRepository,
    authorityRepository: ApplicationAuthorityRepository,
    featureGate: FeatureGate
  ) {
    self.watchZoneRepository = watchZoneRepository
    self.authorityRepository = authorityRepository
    self.featureGate = featureGate
  }

  // MARK: - Computed properties

  /// Number of loaded watch zones.
  public var zoneCount: Int {
    zones.count
  }

  /// Number of authorities from the last fetch result.
  public var authorityCount: Int {
    authorityResult?.count ?? 0
  }

  /// Whether any watch zones have been loaded.
  public var hasZones: Bool {
    !zones.isEmpty
  }

  /// Whether any authorities have been loaded.
  public var hasAuthorities: Bool {
    !authorities.isEmpty
  }

  // MARK: - Data loading

  /// Loads watch zones and authorities concurrently.
  ///
  /// Both fetches run in parallel via `async let`. If one fails, the other
  /// still populates its section -- the error is set from whichever failed
  /// first (or last, if both fail).
  public func load() async {
    isLoading = true
    error = nil

    let zoneRepo = watchZoneRepository
    let authorityRepo = authorityRepository

    async let zonesTask: [WatchZone] = {
      try await zoneRepo.loadAll()
    }()

    async let authoritiesTask: ApplicationAuthorityResult = {
      try await authorityRepo.fetchAuthorities()
    }()

    do {
      zones = try await zonesTask
    } catch {
      handleError(error)
    }

    do {
      let result = try await authoritiesTask
      authorityResult = result
      authorities = result.authorities
    } catch {
      handleError(error)
    }

    isLoading = false
  }

  // MARK: - Navigation

  public func navigateToZones() {
    onNavigateToZones?()
  }

  public func navigateToSaved() {
    onNavigateToSaved?()
  }

  public func navigateToNotifications() {
    onNavigateToNotifications?()
  }

  public func navigateToMap() {
    onNavigateToMap?()
  }

  public func navigateToAuthority(_ authority: LocalAuthority) {
    onNavigateToAuthority?(authority)
  }
}
