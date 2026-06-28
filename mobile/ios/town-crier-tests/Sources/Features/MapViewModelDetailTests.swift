import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("MapViewModel Detail Navigation")
@MainActor
struct MapViewModelDetailTests {
  private func makeSUT(
    applications: [PlanningApplication] = [.pendingReview, .permitted],
    watchZones: [WatchZone] = [.cambridge]
  ) -> (MapViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success(applications)
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success(watchZones)
    let vm = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    return (vm, spy, watchZoneSpy)
  }

  @Test func requestFullDetail_stashesSelectionAsPendingAndClearsSelection() async {
    let (sut, _, _) = makeSUT()
    await sut.loadApplications()
    sut.selectApplication(.pendingReview)

    sut.requestFullDetail()

    #expect(sut.pendingDetailApplication?.id == PlanningApplication.pendingReview.id)
    #expect(sut.selectedApplication == nil)
  }

  @Test func presentPendingDetailIfNeeded_firesCallbackOnceWithPendingApplication() async {
    let (sut, _, _) = makeSUT()
    await sut.loadApplications()
    sut.selectApplication(.pendingReview)
    sut.requestFullDetail()

    var captured: [PlanningApplication] = []
    sut.onShowApplicationDetail = { captured.append($0) }

    sut.presentPendingDetailIfNeeded()
    sut.presentPendingDetailIfNeeded()

    #expect(captured.count == 1)
    #expect(captured.first?.id == PlanningApplication.pendingReview.id)
    #expect(sut.pendingDetailApplication == nil)
  }

  @Test func requestFullDetail_isNoOp_whenNothingSelected() async {
    let (sut, _, _) = makeSUT()
    await sut.loadApplications()

    var captured: [PlanningApplication] = []
    sut.onShowApplicationDetail = { captured.append($0) }

    sut.requestFullDetail()
    sut.presentPendingDetailIfNeeded()

    #expect(sut.pendingDetailApplication == nil)
    #expect(sut.selectedApplication == nil)
    #expect(captured.isEmpty)
  }
}
