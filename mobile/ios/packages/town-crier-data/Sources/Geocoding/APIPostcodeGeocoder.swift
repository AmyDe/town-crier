import Foundation
import TownCrierDomain

/// Geocodes UK postcodes via the Town Crier API's `/v1/geocode/{postcode}` endpoint.
public final class APIPostcodeGeocoder: PostcodeGeocoder, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func geocode(_ postcode: Postcode) async throws -> Coordinate {
    let response: GeocodeResponse
    do {
      response = try await apiClient.request(
        .get("/v1/geocode/\(postcode.value)")
      )
    } catch let domainError as DomainError {
      throw domainError
    } catch {
      throw DomainError.geocodingFailed(postcode.value)
    }
    return try Coordinate(
      latitude: response.coordinates.latitude,
      longitude: response.coordinates.longitude
    )
  }
}

// MARK: - Response DTO

struct GeocodeResponse: Decodable, Sendable {
  let coordinates: CoordinatesDTO
}

struct CoordinatesDTO: Decodable, Sendable {
  let latitude: Double
  let longitude: Double
}
