import Foundation
import TownCrierDomain
import os

/// Persists and retrieves watch zones via the Town Crier API.
public final class APIWatchZoneRepository: WatchZoneRepository, Sendable {
  #if DEBUG
    private static let logger = Logger(subsystem: "uk.towncrierapp", category: "WatchZoneRepository")
  #endif

  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func save(_ zone: WatchZone) async throws {
    let body = CreateWatchZoneRequest(
      name: zone.name,
      latitude: zone.centre.latitude,
      longitude: zone.centre.longitude,
      radiusMetres: zone.radiusMetres,
      authorityId: zone.authorityId > 0 ? zone.authorityId : nil
    )
    do {
      let _: EmptyResponse = try await apiClient.request(.post("/v1/me/watch-zones", body: body))
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }

  public func update(_ zone: WatchZone) async throws {
    let body = UpdateWatchZoneRequest(
      name: zone.name,
      latitude: zone.centre.latitude,
      longitude: zone.centre.longitude,
      radiusMetres: zone.radiusMetres
    )
    do {
      let _: EmptyResponse = try await apiClient.request(
        .patch("/v1/me/watch-zones/\(zone.id.value)", body: body)
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
  }

  public func loadAll() async throws -> [WatchZone] {
    let result: ListWatchZonesResponse
    do {
      result = try await apiClient.request(.get("/v1/me/watch-zones"))
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw error.toDomainError()
    }
    return result.zones.compactMap { dto in
      do {
        return try dto.toDomain()
      } catch {
        #if DEBUG
          Self.logger.warning(
            "Skipping zone '\(dto.id)' ('\(dto.name)'): \(error.localizedDescription)"
          )
        #endif
        return nil
      }
    }
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
      throw error.toDomainError()
    }
  }
}

// MARK: - Request/Response DTOs

struct CreateWatchZoneRequest: Encodable, Sendable {
  let name: String
  let latitude: Double
  let longitude: Double
  let radiusMetres: Double
  let authorityId: Int?
}

struct UpdateWatchZoneRequest: Encodable, Sendable {
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

  func toDomain() throws -> WatchZone {
    let centre = try Coordinate(latitude: latitude, longitude: longitude)
    return try WatchZone(
      id: WatchZoneId(id),
      name: name,
      centre: centre,
      radiusMetres: radiusMetres,
      authorityId: authorityId
    )
  }
}
