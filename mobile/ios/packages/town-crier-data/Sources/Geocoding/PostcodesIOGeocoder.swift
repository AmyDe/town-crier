import Foundation
import TownCrierDomain

/// Geocodes UK postcodes by calling postcodes.io directly from the device,
/// never via our own `/v1/geocode/{postcode}`: the anonymous browse flow has
/// no Auth0 session, and exposing that proxy anonymously would funnel
/// unmetered lookups through our single server IP (blacklist risk). Errors
/// map to the same ``DomainError/geocodingFailed(_:)`` case
/// ``APIPostcodeGeocoder`` uses.
public final class PostcodesIOGeocoder: PostcodeGeocoder, Sendable {
  private let baseURL: URL
  private let transport: HTTPTransport
  private let decoder: JSONDecoder

  public init(
    baseURL: URL = PostcodesIOGeocoder.defaultBaseURL,
    transport: HTTPTransport = URLSession.shared
  ) {
    self.baseURL = baseURL
    self.transport = transport
    self.decoder = JSONDecoder()
  }

  public func geocode(_ postcode: Postcode) async throws -> Coordinate {
    var request = URLRequest(
      url: baseURL.appendingPathComponent("postcodes").appendingPathComponent(postcode.value))
    request.setValue("application/json", forHTTPHeaderField: "Accept")

    let data: Data
    let response: URLResponse
    do {
      (data, response) = try await transport.data(for: request)
    } catch {
      throw DomainError.networkUnavailable
    }

    guard let httpResponse = response as? HTTPURLResponse, httpResponse.statusCode == 200,
      let decoded = try? decoder.decode(PostcodesIOResponse.self, from: data)
    else {
      throw DomainError.geocodingFailed(postcode.value)
    }

    return try Coordinate(
      latitude: decoded.result.latitude, longitude: decoded.result.longitude)
  }

  // swiftlint:disable:next force_unwrapping
  public static let defaultBaseURL = URL(string: "https://api.postcodes.io")!
}

// MARK: - Response DTO

/// Wire shape of postcodes.io's `GET /postcodes/{postcode}` success response.
/// A not-found or malformed postcode instead returns `{"status":404,
/// "error":"Postcode not found"}` (no `result` key), which simply fails to
/// decode against this shape and is mapped to `.geocodingFailed` above.
struct PostcodesIOResponse: Decodable, Sendable {
  let result: PostcodesIOResult
}

struct PostcodesIOResult: Decodable, Sendable {
  let latitude: Double
  let longitude: Double
}
