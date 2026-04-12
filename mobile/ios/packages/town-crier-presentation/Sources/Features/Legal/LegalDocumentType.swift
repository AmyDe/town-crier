/// The type of legal document available for in-app display.
public enum LegalDocumentType: String, Sendable, Identifiable {
  case privacyPolicy
  case termsOfService

  public var id: String { rawValue }
}
