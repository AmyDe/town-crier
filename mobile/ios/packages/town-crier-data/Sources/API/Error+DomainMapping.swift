import Foundation
import TownCrierDomain

/// Maps API-layer and network-layer errors to the appropriate `DomainError` case.
///
/// Repositories use this in their catch-all blocks instead of blanket-mapping
/// every error to `.networkUnavailable`. Only actual `URLError` connectivity
/// failures map to `.networkUnavailable`; HTTP errors map to `.serverError`
/// or `.unexpected` as appropriate.
extension Error {
  func toDomainError() -> DomainError {
    if let apiError = self as? APIError {
      switch apiError {
      case let .serverError(statusCode, message):
        return .serverError(statusCode: statusCode, message: message)
      case .decodingFailed(let detail):
        return .unexpected(detail)
      case .unauthorized:
        return .sessionExpired
      case .notFound:
        return .unexpected("Resource not found")
      }
    }
    if self is URLError {
      return .networkUnavailable
    }
    return .unexpected(localizedDescription)
  }
}
