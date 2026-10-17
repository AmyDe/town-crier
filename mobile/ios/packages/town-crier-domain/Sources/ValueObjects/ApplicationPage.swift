/// One page of watch-zone planning applications plus the opaque continuation
/// token for the next page. `nextCursor` is `nil` on the last page.
public struct ApplicationPage: Sendable, Equatable {
  public let applications: [PlanningApplication]
  public let nextCursor: String?

  public init(applications: [PlanningApplication], nextCursor: String?) {
    self.applications = applications
    self.nextCursor = nextCursor
  }
}
