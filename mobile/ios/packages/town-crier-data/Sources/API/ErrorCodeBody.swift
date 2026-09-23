import Foundation

/// DTO for parsing the `{error, message}` envelope several endpoints return on
/// `400`/`409` (watch-zone save, subscription verify).
///
/// The API returns e.g. `{"error": "boundary_too_large", "message": "..."}`
/// or `{"error": "transaction_already_claimed", "message": "..."}`. Internal
/// to the data layer -- callers see the mapped `DomainError` case (GH#1085,
/// GH#1165).
struct ErrorCodeBody: Decodable, Sendable {
  let error: String
  let message: String?
}
