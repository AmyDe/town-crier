import Foundation

/// A planning application that the user has bookmarked for later reference.
public struct SavedApplication: Equatable, Sendable {
  public let applicationUid: String
  public let savedAt: Date
  public let application: PlanningApplication?

  public init(
    applicationUid: String,
    savedAt: Date,
    application: PlanningApplication? = nil
  ) {
    self.applicationUid = applicationUid
    self.savedAt = savedAt
    self.application = application
  }
}
