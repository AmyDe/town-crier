import Foundation

/// DTO for the `{error, message}` envelope returned by watch-zone and subscription endpoints on `400`/`409`.
struct ErrorCodeBody: Decodable, Sendable {
  let error: String
  let message: String?
}
