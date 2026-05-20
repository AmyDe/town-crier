import Foundation
import TownCrierDomain

/// HTTP adapter for `SubscriptionVerificationService` that POSTs an Apple-signed
/// StoreKit 2 transaction to `POST /v1/subscriptions/verify`.
///
/// The request carries the compact JWS string verbatim; the backend
/// cryptographically verifies it against Apple's certificate chain and writes
/// the resolved tier/expiry to the user's Cosmos `UserProfile` (ADR 0010).
public final class HttpSubscriptionVerificationService: SubscriptionVerificationService, Sendable {
  private let apiClient: URLSessionAPIClient

  public init(apiClient: URLSessionAPIClient) {
    self.apiClient = apiClient
  }

  public func verify(signedTransaction: String) async throws -> VerifiedSubscription {
    let dto: VerifyResponseDTO = try await apiClient.request(
      .post(
        "/v1/subscriptions/verify",
        body: VerifyRequestDTO(signedTransaction: signedTransaction)
      )
    )

    let tier = SubscriptionTier(rawValue: dto.tier.lowercased()) ?? .free

    var expiry: Date?
    if let raw = dto.subscriptionExpiry {
      let formatter = ISO8601DateFormatter()
      formatter.formatOptions = [.withInternetDateTime]
      expiry = formatter.date(from: raw)
        ?? {
          formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
          return formatter.date(from: raw)
        }()
    }

    return VerifiedSubscription(
      tier: tier,
      subscriptionExpiry: expiry,
      entitlements: dto.entitlements,
      watchZoneLimit: dto.watchZoneLimit
    )
  }
}

// MARK: - DTOs

private struct VerifyRequestDTO: Encodable, Sendable {
  let signedTransaction: String
}

private struct VerifyResponseDTO: Decodable, Sendable {
  let tier: String
  let subscriptionExpiry: String?
  let entitlements: [String]
  let watchZoneLimit: Int
}
