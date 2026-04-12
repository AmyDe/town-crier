import Foundation
import TownCrierDomain

/// Decodes JWT payloads and extracts custom claims.
///
/// Performs base64url decoding of the JWT payload segment without
/// signature verification -- tokens are already verified by Auth0 SDK.
enum JWTSubscriptionTierExtractor {

  /// Extracts the `subscription_tier` custom claim from a JWT access token.
  /// Returns `.free` when the claim is absent, unrecognised, or the token
  /// is malformed.
  static func extractTier(from accessToken: String) -> SubscriptionTier {
    guard let payload = decodePayload(from: accessToken),
      let tierString = payload["subscription_tier"] as? String,
      let tier = SubscriptionTier(rawValue: tierString)
    else {
      return .free
    }
    return tier
  }

  /// Extracts the `sub` claim from a JWT token.
  /// Returns `nil` when the claim is absent or the token is malformed.
  static func extractSubject(from token: String) -> String? {
    guard let payload = decodePayload(from: token) else { return nil }
    return payload["sub"] as? String
  }

  /// Decodes the payload segment of a JWT token into a dictionary.
  /// Returns `nil` if the token is malformed or the payload is not valid JSON.
  static func decodePayload(from token: String) -> [String: Any]? {
    let segments = token.split(separator: ".")
    guard segments.count >= 2 else { return nil }

    var base64 = String(segments[1])
      .replacingOccurrences(of: "-", with: "+")
      .replacingOccurrences(of: "_", with: "/")

    let remainder = base64.count % 4
    if remainder > 0 {
      base64 += String(repeating: "=", count: 4 - remainder)
    }

    guard let data = Data(base64Encoded: base64) else { return nil }
    return try? JSONSerialization.jsonObject(with: data) as? [String: Any]
  }
}
