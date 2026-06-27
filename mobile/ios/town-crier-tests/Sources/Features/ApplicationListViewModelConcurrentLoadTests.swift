import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Guards against the client-side retry storm in bd tc-eum5: a single user
/// action (`.task` firing alongside `.refreshable`, scenePhase changes, or a
/// view re-appearing) must issue at most ONE in-flight repository fetch for the
/// watch-zone applications list. Without the guard the same
/// `GET /v1/me/watch-zones/{id}/applications` request fired 3-6 times within
/// seconds, all cancelled (HTTP 499).
@Suite("ApplicationListViewModel — concurrent load guard")
@MainActor
struct ApplicationListViewModelConcurrentLoadTests {

  @Test func loadApplications_whileLoadInFlight_doesNotIssueSecondFetch() async {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success([.pendingReview])
    spy.enableGate()
    let sut = ApplicationListViewModel(repository: spy, zone: .cambridge)

    // First load parks inside the gated fetch (fetch call #1 recorded).
    let first = Task { await sut.loadApplications() }
    await Task.yield()
    #expect(sut.isLoading)

    // Second concurrent call must short-circuit synchronously — its
    // re-entrancy guard runs before any `await`, so it never reaches the
    // repository. Issued as a Task so it can't deadlock the test if the guard
    // is missing.
    let second = Task { await sut.loadApplications() }
    await Task.yield()
    await Task.yield()

    // The guard means the in-flight fetch count is still exactly 1. The list is
    // paged for every sort now (GH#682 slice 3), so the load drives the paged
    // endpoint rather than the param-less fetch.
    #expect(spy.fetchApplicationsPageCalls.count == 1)

    spy.releaseGate()
    await first.value
    await second.value

    #expect(spy.fetchApplicationsPageCalls.count == 1)
  }

  @Test func loadApplications_afterPriorLoadCompletes_fetchesAgain() async {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success([.pendingReview])
    let sut = ApplicationListViewModel(repository: spy, zone: .cambridge)

    await sut.loadApplications()
    await sut.loadApplications()

    // Sequential (non-overlapping) loads are still allowed — pull-to-refresh
    // after the first load completes must hit the network.
    #expect(spy.fetchApplicationsPageCalls.count == 2)
  }
}
