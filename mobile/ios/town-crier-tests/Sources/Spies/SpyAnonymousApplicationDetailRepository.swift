import TownCrierDomain

final class SpyAnonymousApplicationDetailRepository: AnonymousApplicationDetailRepository,
  @unchecked Sendable {
  struct RecordedBySlugRequest: Sendable, Equatable {
    let authoritySlug: String
    let ref: String
  }

  private(set) var fetchApplicationBySlugCalls: [RecordedBySlugRequest] = []
  var fetchApplicationBySlugResult: Result<PlanningApplication, Error> = .success(.pendingReview)

  /// Per-`ref` results. When a key matches the requested `ref` it takes
  /// precedence over `fetchApplicationBySlugResult`, so a stacked-cluster
  /// test can return a distinct application per member (and assert ordering
  /// / identities) or make a single member's read throw to prove the
  /// all-or-nothing resilience of `AnonymousMapViewModel.selectStack(_:)`
  /// (mirrors `SpyPlanningApplicationRepository.fetchApplicationResultsById`).
  var fetchApplicationBySlugResultsByRef: [String: Result<PlanningApplication, Error>] = [:]

  func fetchApplication(bySlug authoritySlug: String, ref: String) async throws
    -> PlanningApplication {
    fetchApplicationBySlugCalls.append(
      RecordedBySlugRequest(authoritySlug: authoritySlug, ref: ref))
    if let scoped = fetchApplicationBySlugResultsByRef[ref] {
      return try scoped.get()
    }
    return try fetchApplicationBySlugResult.get()
  }
}
