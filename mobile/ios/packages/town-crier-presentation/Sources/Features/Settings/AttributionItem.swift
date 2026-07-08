import Foundation

/// A data source attribution entry for display in settings.
public struct AttributionItem: Equatable, Sendable {
  public let name: String
  public let detail: String
  public let url: URL?

  public init(name: String, detail: String, url: URL? = nil) {
    self.name = name
    self.detail = detail
    self.url = url
  }
}

extension AttributionItem {
  /// Standard data-source attribution rows shown in both the authenticated
  /// Settings screen and the anonymous Settings tab (GH#879 Phase 3) — kept
  /// in one place so the two surfaces cannot drift apart.
  public static let standard: [AttributionItem] = [
    AttributionItem(
      name: "PlanIt",
      detail: "Planning application data",
      url: URL(string: "https://www.planit.org.uk")
    ),
    AttributionItem(
      name: "Crown Copyright",
      detail: "Contains public sector information"
    ),
    AttributionItem(
      name: "Ordnance Survey",
      detail: "Mapping data"
    ),
    AttributionItem(
      name: "Apple Maps",
      detail: "Map rendering and geocoding",
      url: URL(string: "https://www.apple.com/maps/")
    ),
  ]
}
