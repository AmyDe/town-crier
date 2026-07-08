import Foundation
import TownCrierDomain

final class SpyAnonymousApplicationsRepository: AnonymousApplicationsRepository, @unchecked Sendable {
  struct FetchNearbyCall: Equatable {
    let latitude: Double
    let longitude: Double
    let radiusMetres: Double
    let limit: Int
  }

  private(set) var fetchNearbyCalls: [FetchNearbyCall] = []
  var fetchNearbyResult: Result<[PlanningApplication], Error> = .success([])

  func fetchNearby(
    latitude: Double, longitude: Double, radiusMetres: Double, limit: Int
  ) async throws -> [PlanningApplication] {
    fetchNearbyCalls.append(
      FetchNearbyCall(latitude: latitude, longitude: longitude, radiusMetres: radiusMetres, limit: limit))
    return try fetchNearbyResult.get()
  }
}
