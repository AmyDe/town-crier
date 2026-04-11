import Foundation

/// A validated UK postcode.
public struct Postcode: Equatable, Hashable, Sendable {
  public let value: String

  public init(_ raw: String) throws {
    let trimmed = raw.trimmingCharacters(in: .whitespaces).uppercased()
    guard Self.isValid(trimmed) else {
      throw DomainError.invalidPostcode(raw)
    }
    value = trimmed
  }

  private static func isValid(_ postcode: String) -> Bool {
    let pattern = #"^[A-Z]{1,2}\d[A-Z\d]?\s?\d[A-Z]{2}$"#
    return postcode.range(of: pattern, options: .regularExpression) != nil
  }
}
