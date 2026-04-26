import Foundation

/// ViewModel for displaying a legal document (privacy policy or terms of service).
/// Content is loaded from the JSON files bundled in `Bundle.module`. The same JSON
/// files are embedded on the API side (canonical source); CI enforces byte-equality.
public struct LegalDocumentViewModel: Sendable {
  public let title: String
  public let lastUpdated: String
  public let sections: [LegalDocumentSection]
  public let documentType: LegalDocumentType

  public init(documentType: LegalDocumentType) {
    self.documentType = documentType
    let resourceName = Self.resourceName(for: documentType)
    let decoded = Self.loadBundledDocument(named: resourceName)

    title = decoded.title
    lastUpdated = Self.formatLastUpdated(decoded.lastUpdated)
    sections = decoded.sections.map { section in
      LegalDocumentSection(heading: section.heading, body: section.body)
    }
  }

  private static func resourceName(for documentType: LegalDocumentType) -> String {
    switch documentType {
    case .privacyPolicy:
      "privacy"
    case .termsOfService:
      "terms"
    }
  }

  private static func loadBundledDocument(named name: String) -> LegalDocumentJSON {
    guard let url = Bundle.module.url(forResource: name, withExtension: "json") else {
      fatalError("legal docs missing in bundle: \(name).json")
    }
    do {
      let data = try Data(contentsOf: url)
      return try JSONDecoder().decode(LegalDocumentJSON.self, from: data)
    } catch {
      fatalError("legal docs corrupt in bundle (\(name).json): \(error)")
    }
  }

  private static func formatLastUpdated(_ iso: String) -> String {
    let isoFormatter = ISO8601DateFormatter()
    isoFormatter.formatOptions = [.withFullDate]
    guard let date = isoFormatter.date(from: iso) else { return iso }

    let displayFormatter = DateFormatter()
    displayFormatter.locale = Locale(identifier: "en_GB")
    displayFormatter.dateFormat = "d MMMM yyyy"
    return displayFormatter.string(from: date)
  }
}

// MARK: - JSON Decoding

private struct LegalDocumentJSON: Decodable {
  let title: String
  let lastUpdated: String
  let sections: [Section]

  struct Section: Decodable {
    let heading: String
    let body: String
  }
}
