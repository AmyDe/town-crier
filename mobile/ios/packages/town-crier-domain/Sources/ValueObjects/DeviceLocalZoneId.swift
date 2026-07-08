import Foundation

/// Unique identifier for a device-local zone (GH#879 Phase 4).
public struct DeviceLocalZoneId: Equatable, Hashable, Sendable {
  public let value: String

  public init(_ value: String) {
    self.value = value
  }

  public init() {
    self.value = UUID().uuidString
  }
}
