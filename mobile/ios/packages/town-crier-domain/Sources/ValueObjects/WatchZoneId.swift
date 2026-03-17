import Foundation

/// Unique identifier for a watch zone.
public struct WatchZoneId: Equatable, Hashable, Sendable {
    public let value: String

    public init(_ value: String) {
        self.value = value
    }

    public init() {
        self.value = UUID().uuidString
    }
}
