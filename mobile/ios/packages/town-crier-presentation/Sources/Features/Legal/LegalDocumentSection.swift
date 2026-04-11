/// A titled section within a legal document.
public struct LegalDocumentSection: Sendable, Equatable {
  public let heading: String
  public let body: String

  public init(heading: String, body: String) {
    self.heading = heading
    self.body = body
  }
}
