import Foundation

/// DTO for parsing 403 responses from the API when the user lacks an entitlement.
///
/// The API returns `{"error": "insufficient_entitlement", "required": "<entitlement>"}`.
/// Internal to the data layer -- callers see `DomainError.insufficientEntitlement`.
struct InsufficientEntitlementBody: Decodable, Sendable {
  let error: String
  let required: String
}
