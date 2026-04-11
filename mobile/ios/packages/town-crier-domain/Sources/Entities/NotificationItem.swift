import Foundation

/// A notification about a planning application event delivered to the user.
public struct NotificationItem: Equatable, Sendable {
  public let applicationName: String
  public let applicationAddress: String
  public let applicationDescription: String
  public let applicationType: String
  public let authorityId: Int
  public let createdAt: Date

  public init(
    applicationName: String,
    applicationAddress: String,
    applicationDescription: String,
    applicationType: String,
    authorityId: Int,
    createdAt: Date
  ) {
    self.applicationName = applicationName
    self.applicationAddress = applicationAddress
    self.applicationDescription = applicationDescription
    self.applicationType = applicationType
    self.authorityId = authorityId
    self.createdAt = createdAt
  }
}
