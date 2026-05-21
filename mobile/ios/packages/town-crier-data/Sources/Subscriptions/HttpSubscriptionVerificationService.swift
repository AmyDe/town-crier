import Foundation
import TownCrierDomain

/// HTTP adapter for `SubscriptionVerificationService` that POSTs Apple-signed
/// StoreKit 2 transactions to `POST /v1/subscriptions/verify`.
///
/// The request carries the compact JWS string(s) verbatim; the backend
/// cryptographically verifies them against Apple's certificate chain and writes
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
    return Self.verifiedSubscription(from: dto)
  }

  public func verifyRestore(signedTransactions: [String]) async throws -> VerifiedSubscription {
    let dto: VerifyResponseDTO = try await apiClient.request(
      .post(
        "/v1/subscriptions/verify",
        body: RestoreRequestDTO(signedTransactions: signedTransactions)
      )
    )
    return Self.verifiedSubscription(from: dto)
  }

  // MARK: - Response mapping

  private static func verifiedSubscription(from dto: VerifyResponseDTO) -> VerifiedSubscription {
    let tier = SubscriptionTier(rawValue: dto.tier.lowercased()) ?? .free

    var expiry: Date?
    if let raw = dto.subscriptionExpiry {
      let formatter = ISO8601DateFormatter()
      formatter.formatOptions = [.withInternetDateTime]
      expiry =
        formatter.date(from: raw)
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

/// Purchase request body: `{ "signedTransaction": "<jws>" }`.
private struct VerifyRequestDTO: Encodable, Sendable {
  let signedTransaction: String
}

/// Restore request body: `{ "signedTransactions": ["<jws1>", "<jws2>", ...] }`
/// — the JWS strings from `Transaction.currentEntitlements`.
private struct RestoreRequestDTO: Encodable, Sendable {
  let signedTransactions: [String]
}

private struct VerifyResponseDTO: Decodable, Sendable {
  let tier: String
  let subscriptionExpiry: String?
  let entitlements: [String]
  let watchZoneLimit: Int
}
