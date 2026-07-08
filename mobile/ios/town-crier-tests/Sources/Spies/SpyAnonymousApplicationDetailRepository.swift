import TownCrierDomain

final class SpyAnonymousApplicationDetailRepository: AnonymousApplicationDetailRepository,
  @unchecked Sendable {
  struct RecordedBySlugRequest: Sendable, Equatable {
    let authoritySlug: String
    let ref: String
  }

  private(set) var fetchApplicationBySlugCalls: [RecordedBySlugRequest] = []
  var fetchApplicationBySlugResult: Result<PlanningApplication, Error> = .success(.pendingReview)

  func fetchApplication(bySlug authoritySlug: String, ref: String) async throws
    -> PlanningApplication {
    fetchApplicationBySlugCalls.append(
      RecordedBySlugRequest(authoritySlug: authoritySlug, ref: ref))
    return try fetchApplicationBySlugResult.get()
  }
}
