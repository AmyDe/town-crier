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
