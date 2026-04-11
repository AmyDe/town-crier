import Foundation
import TownCrierDomain

/// Extracts the `subscription_tier` custom claim from a JWT access token.
///
/// The Auth0 Post-Login Action adds `subscription_tier` to the access token.
/// This extractor decodes the JWT payload (without signature verification --
/// the token has already been verified by Auth0 SDK) and maps the claim to
/// a `SubscriptionTier` value. Returns `.free` when the claim is absent,
/// unrecognised, or the token is malformed.
enum JWTSubscriptionTierExtractor {

  static func extractTier(from accessToken: String) -> SubscriptionTier {
    guard let payload = decodePayload(from: accessToken),
      let tierString = payload["subscription_tier"] as? String,
      let tier = SubscriptionTier(rawValue: tierString)
    else {
      return .free
    }
    return tier
  }

  private static func decodePayload(
    from token: String
  ) -> [String: Any]? {
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
