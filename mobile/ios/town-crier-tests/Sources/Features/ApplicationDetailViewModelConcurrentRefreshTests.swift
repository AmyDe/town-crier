import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Guards against the client-side retry storm in bd tc-eum5: a single detail
/// open (`.task` firing alongside scenePhase changes or a re-appear) must issue
/// at most ONE in-flight `GET /v1/applications/{ref}` fetch. Without the guard
/// the same request fired up to 6 times within seconds, all cancelled
/// (HTTP 499).
@Suite("ApplicationDetailViewModel — concurrent refresh guard")
@MainActor
struct ApplicationDetailViewModelConcurrentRefreshTests {

  @Test func refresh_whileRefreshInFlight_doesNotIssueSecondFetch() async {
    let repo = SpyPlanningApplicationRepository()
    repo.fetchApplicationResult = .success(.permitted)
    repo.enableGate()
    let sut = ApplicationDetailViewModel(
      application: .pendingReview,
      planningApplicationRepository: repo
    )

    // First refresh parks inside the gated fetch (fetch call #1 recorded).
    let first = Task { await sut.refresh() }
    await Task.yield()

    // Second concurrent refresh must short-circuit synchronously — its
    // re-entrancy guard runs before any `await`, so it never reaches the
    // repository. Issued as a Task so it can't deadlock the test if the guard
    // is missing.
    let second = Task { await sut.refresh() }
    await Task.yield()
    await Task.yield()

    #expect(repo.fetchApplicationCalls.count == 1)

    repo.releaseGate()
    await first.value
    await second.value

    #expect(repo.fetchApplicationCalls.count == 1)
  }

  @Test func refresh_afterPriorRefreshCompletes_fetchesAgain() async {
    let repo = SpyPlanningApplicationRepository()
    repo.fetchApplicationResult = .success(.pendingReview)
    let sut = ApplicationDetailViewModel(
      application: .pendingReview,
      planningApplicationRepository: repo
    )

    await sut.refresh()
    await sut.refresh()

    // Sequential refreshes still fire — each detail open must revalidate.
    #expect(repo.fetchApplicationCalls.count == 2)
  }
}
