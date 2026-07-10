import Foundation
import TownCrierDomain

final class SpyAnonymousApplicationsRepository: AnonymousApplicationsRepository, @unchecked Sendable {
  struct FetchNearbyCall: Equatable {
    let latitude: Double
    let longitude: Double
    let radiusMetres: Double
    let limit: Int
    let sort: NearbyApplicationSortOrder
  }

  private(set) var fetchNearbyCalls: [FetchNearbyCall] = []
  var fetchNearbyResult: Result<[PlanningApplication], Error> = .success([])

  func fetchNearby(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    limit: Int,
    sort: NearbyApplicationSortOrder
  ) async throws -> [PlanningApplication] {
    fetchNearbyCalls.append(
      FetchNearbyCall(
        latitude: latitude,
        longitude: longitude,
        radiusMetres: radiusMetres,
        limit: limit,
        sort: sort))
    return try fetchNearbyResult.get()
  }

  // MARK: - Cluster fetch (GH#924 Phase 2)

  struct FetchClustersCall: Equatable {
    let latitude: Double
    let longitude: Double
    let radiusMetres: Double
    let viewport: MapViewport
    let zoom: Int
  }

  private(set) var fetchClustersCalls: [FetchClustersCall] = []
  var fetchClustersResult: Result<[AnonymousMapCluster], Error> = .success([])

  func fetchClusters(
    latitude: Double,
    longitude: Double,
    radiusMetres: Double,
    viewport: MapViewport,
    zoom: Int
  ) async throws -> [AnonymousMapCluster] {
    fetchClustersCalls.append(
      FetchClustersCall(
        latitude: latitude,
        longitude: longitude,
        radiusMetres: radiusMetres,
        viewport: viewport,
        zoom: zoom))
    return try fetchClustersResult.get()
  }
}
