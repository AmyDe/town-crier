/// A unique identifier for a planning application, composed of the authority
/// (area ID as a decimal string) and the PlanIt case reference.
///
/// Using a struct of two fields rather than a single concatenated string keeps
/// the parts inseparable at the type level and eliminates the ambiguity between
/// a bare case reference and an authority-prefixed variant (tc-dzwo.1).
public struct PlanningApplicationId: Equatable, Hashable, Sendable {
  /// The local authority area ID, expressed as a decimal string (e.g. `"42"`).
  public let authority: String
  /// The PlanIt case reference (e.g. `"22/1234/FUL"`).
  public let name: String

  public init(authority: String, name: String) {
    self.authority = authority
    self.name = name
  }

  /// The canonical slash-joined string form of the identifier
  /// (e.g. `"42/22/1234/FUL"`). Used wherever a single-string representation
  /// is required — saved-application paths, server-side UIDs, and display.
  public var value: String {
    "\(authority)/\(name)"
  }
}
