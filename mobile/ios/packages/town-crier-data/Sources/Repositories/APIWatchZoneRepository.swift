import Foundation
import TownCrierDomain

/// Persists and retrieves watch zones via the Town Crier API.
public final class APIWatchZoneRepository: WatchZoneRepository, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func save(_ zone: WatchZone) async throws {
    let body = CreateWatchZoneRequest(
      zoneId: zone.id.value,
      name: zone.postcode.value,
      latitude: zone.centre.latitude,
      longitude: zone.centre.longitude,
      radiusMetres: zone.radiusMetres
    )
    do {
      let _: EmptyResponse = try await apiClient.request(.post("/v1/me/watch-zones", body: body))
    } catch is URLError {
      throw DomainError.networkUnavailable
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw DomainError.networkUnavailable
    }
  }

  public func loadAll() async throws -> [WatchZone] {
    let result: ListWatchZonesResponse
    do {
      result = try await apiClient.request(.get("/v1/me/watch-zones"))
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw DomainError.networkUnavailable
    }
    return result.zones.compactMap { $0.toDomain() }
  }

  public func delete(_ id: WatchZoneId) async throws {
    do {
      let _: EmptyResponse = try await apiClient.request(
        .delete("/v1/me/watch-zones/\(id.value)")
      )
    } catch APIError.notFound {
      // Idempotent delete — if the zone is already gone, succeed silently
      return
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw DomainError.networkUnavailable
    }
  }
}

// MARK: - Request/Response DTOs

struct CreateWatchZoneRequest: Encodable, Sendable {
  let zoneId: String
  let name: String
  let latitude: Double
  let longitude: Double
  let radiusMetres: Double
}

struct ListWatchZonesResponse: Decodable, Sendable {
  let zones: [WatchZoneSummaryDTO]
}

struct WatchZoneSummaryDTO: Decodable, Sendable {
  let id: String
  let name: String
  let latitude: Double
  let longitude: Double
  let radiusMetres: Double
  let authorityId: Int

  func toDomain() -> WatchZone? {
    guard let postcode = try? Postcode(name),
      let centre = try? Coordinate(latitude: latitude, longitude: longitude)
    else {
      return nil
    }
    return try? WatchZone(
      id: WatchZoneId(id),
      postcode: postcode,
      centre: centre,
      radiusMetres: radiusMetres
    )
  }
}
