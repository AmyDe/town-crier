import Foundation

/// A lightweight representation of a crash diagnostic report.
public struct CrashReport: Equatable, Sendable {
  public let id: String
  public let timestamp: Date
  public let signal: String
  public let reason: String
  public let terminationDescription: String

  public init(
    id: String,
    timestamp: Date,
    signal: String,
    reason: String,
    terminationDescription: String
  ) {
    self.id = id
    self.timestamp = timestamp
    self.signal = signal
    self.reason = reason
    self.terminationDescription = terminationDescription
  }
}
