import Foundation
import TownCrierDomain

final class SpyAnonymousApplicationDetailRepository: AnonymousApplicationDetailRepository,
  @unchecked Sendable {
  struct RecordedBySlugRequest: Sendable, Equatable {
    let authoritySlug: String
    let ref: String
  }

  /// Guards ``fetchApplicationBySlugCalls``'s mutation below. Concurrent
  /// callers are expected: `AnonymousMapViewModel.selectStack(_:)` point-reads
  /// every stacked member via a `withThrowingTaskGroup`, so several test
  /// cases call this spy from multiple tasks at once — a plain `Array.append`
  /// with no synchronization intermittently dropped an entry under `swift
  /// test`'s default parallel execution (GH#924 review).
  private let lock = NSLock()

  private var _fetchApplicationBySlugCalls: [RecordedBySlugRequest] = []
  var fetchApplicationBySlugCalls: [RecordedBySlugRequest] {
    lock.withLock { _fetchApplicationBySlugCalls }
  }

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
    lock.withLock {
      _fetchApplicationBySlugCalls.append(
        RecordedBySlugRequest(authoritySlug: authoritySlug, ref: ref))
    }
    if let scoped = fetchApplicationBySlugResultsByRef[ref] {
      return try scoped.get()
    }
    return try fetchApplicationBySlugResult.get()
  }
}
