import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Covers the stacked-cluster (unsplittable, coincident-members) tap path
/// (GH#722): `selectStack(_:)` point-reads every member, publishes the
/// disambiguation list in member order, leaves the map untouched on any failure,
/// and the row-select → pending → summary handoff that keeps one sheet on screen.
@Suite("MapViewModel — stacked clusters")
@MainActor
struct MapViewModelStackTests {
  private func makeSUT(
    clusters: [MapCluster] = []
  ) -> (MapViewModel, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchClustersResult = .success(clusters)
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([.cambridge])
    let vm = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    return (vm, spy)
  }

  @Test func selectStack_fetchesEachMemberAndPublishesOrderedList() async {
    let (sut, spy) = makeSUT(clusters: [.bubble(count: 3)])
    await sut.loadApplications()

    let members = [
      PlanningApplication.pendingReview.id,
      PlanningApplication.permitted.id,
      PlanningApplication.rejected.id,
    ]
    spy.fetchApplicationResultsById = [
      PlanningApplication.pendingReview.id: .success(.pendingReview),
      PlanningApplication.permitted.id: .success(.permitted),
      PlanningApplication.rejected.id: .success(.rejected),
    ]

    await sut.selectStack(.stacked(members: members))

    // Publishes one application per member, in the cluster's member order
    // (a TaskGroup completes out of order, so the result must be reindexed).
    #expect(sut.stackedApplications?.applications.map(\.id) == members)
    // Every member was point-read exactly once.
    #expect(spy.fetchApplicationCalls.count == 3)
    #expect(Set(spy.fetchApplicationCalls) == Set(members))
    #expect(sut.selectedApplication == nil)
  }

  @Test func selectStack_leavesMapUntouched_whenAMemberReadFails() async {
    let (sut, spy) = makeSUT(clusters: [.bubble(count: 3)])
    await sut.loadApplications()
    #expect(sut.clusters.count == 1)

    let members = [PlanningApplication.pendingReview.id, PlanningApplication.permitted.id]
    spy.fetchApplicationResultsById = [
      PlanningApplication.pendingReview.id: .success(.pendingReview),
      PlanningApplication.permitted.id: .failure(DomainError.networkUnavailable),
    ]

    await sut.selectStack(.stacked(members: members))

    // All-or-nothing: one failed member publishes no list and never blanks the
    // map with an error — the user can tap again (mirrors selectCluster).
    #expect(sut.stackedApplications == nil)
    #expect(sut.error == nil)
    #expect(sut.clusters.count == 1)
  }

  @Test func selectStack_nonStackedCluster_doesNothing() async {
    let (sut, spy) = makeSUT()
    await sut.loadApplications()

    await sut.selectStack(.bubble(count: 42))

    #expect(sut.stackedApplications == nil)
    #expect(spy.fetchApplicationCalls.isEmpty)
  }

  @Test func selectFromStack_routesThroughPendingToSummary() async {
    let (sut, spy) = makeSUT(clusters: [.bubble(count: 2)])
    await sut.loadApplications()
    let members = [PlanningApplication.pendingReview.id, PlanningApplication.permitted.id]
    spy.fetchApplicationResultsById = [
      PlanningApplication.pendingReview.id: .success(.pendingReview),
      PlanningApplication.permitted.id: .success(.permitted),
    ]
    await sut.selectStack(.stacked(members: members))

    // Tapping a row stashes the chosen application and dismisses the list — the
    // summary must NOT be up yet (no two sheets at once).
    sut.selectFromStack(.permitted)
    #expect(sut.stackedApplications == nil)
    #expect(sut.selectedApplication == nil)

    // The list sheet's onDismiss then presents the summary.
    sut.presentPendingSummaryIfNeeded()
    #expect(sut.selectedApplication == .permitted)
  }

  @Test func clearStack_clearsStackedApplications() async {
    let (sut, spy) = makeSUT(clusters: [.bubble(count: 2)])
    await sut.loadApplications()
    let members = [PlanningApplication.pendingReview.id, PlanningApplication.permitted.id]
    spy.fetchApplicationResultsById = [
      PlanningApplication.pendingReview.id: .success(.pendingReview),
      PlanningApplication.permitted.id: .success(.permitted),
    ]
    await sut.selectStack(.stacked(members: members))
    #expect(sut.stackedApplications != nil)

    sut.clearStack()

    #expect(sut.stackedApplications == nil)
  }
}
