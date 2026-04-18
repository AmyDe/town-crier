import Foundation
import TownCrierDomain

/// HTTP adapter for `OfferCodeService` that POSTs to `/v1/offer-codes/redeem`.
///
/// The request body is forwarded verbatim — the backend normalises the code
/// (strips `-`, uppercases) so display-formatted input from the UI works
/// without additional client-side massaging. HTTP failures are translated
/// into `OfferCodeError`:
///
/// - 404 → `.notFound` (the server's `invalid_code` response)
/// - 4xx with a parseable `{"error": "..."}` body → the matching typed case
/// - anything else → `.network` with the status code for UI fallback
public final class HttpOfferCodeService: OfferCodeService, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func redeem(code: String) async throws -> OfferCodeRedemption {
    do {
      let dto: RedeemResponseDTO = try await apiClient.request(
        .post("/v1/offer-codes/redeem", body: RedeemRequestDTO(code: code))
      )
      guard let tier = SubscriptionTier(rawValue: dto.tier.lowercased()) else {
        throw OfferCodeError.network("Unknown tier: \(dto.tier)")
      }
      let formatter = ISO8601DateFormatter()
      formatter.formatOptions = [.withInternetDateTime]
      guard let expiresAt = formatter.date(from: dto.expiresAt) else {
        throw OfferCodeError.network("Unparseable expiresAt: \(dto.expiresAt)")
      }
      return OfferCodeRedemption(tier: tier, expiresAt: expiresAt)
    } catch APIError.notFound {
      throw OfferCodeError.notFound
    } catch APIError.serverError(let statusCode, let message) {
      guard (400..<500).contains(statusCode) else {
        throw OfferCodeError.network("HTTP \(statusCode)")
      }
      throw Self.mapErrorBody(message: message, statusCode: statusCode)
    }
  }

  private static func mapErrorBody(message: String?, statusCode: Int) -> OfferCodeError {
    guard
      let message,
      let data = message.data(using: .utf8),
      let body = try? JSONDecoder().decode(RedeemErrorBodyDTO.self, from: data)
    else {
      return .network("HTTP \(statusCode)")
    }
    return OfferCodeError(serverErrorCode: body.error)
  }
}

// MARK: - DTOs

private struct RedeemRequestDTO: Encodable, Sendable {
  let code: String
}

private struct RedeemResponseDTO: Decodable, Sendable {
  let tier: String
  let expiresAt: String
}

private struct RedeemErrorBodyDTO: Decodable, Sendable {
  let error: String
}
