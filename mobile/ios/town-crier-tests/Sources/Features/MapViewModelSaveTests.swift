import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("MapViewModel -- Save from Summary Sheet")
@MainActor
struct MapViewModelSaveTests {
  private static let allApps: [PlanningApplication] = [
    .pendingReview, .approved, .refused, .withdrawn,
  ]

  private func makeSUT(
    applications: [PlanningApplication] = allApps,
    savedApplicationUids: [String] = []
  ) -> (MapViewModel, SpySavedApplicationRepository) {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(applications)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let savedSpy = SpySavedApplicationRepository()
    savedSpy.loadAllResult = .success(
      savedApplicationUids.map { SavedApplication(applicationUid: $0, savedAt: Date()) }
    )
    let vm = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
      savedApplicationRepository: savedSpy
    )
    return (vm, savedSpy)
  }

  // MARK: - isSelectedApplicationSaved

  @Test("isSelectedApplicationSaved is false when no application is selected")
  func isSelectedApplicationSaved_noSelection_returnsFalse() async {
    let (sut, _) = makeSUT()
    await sut.loadApplications()

    #expect(!sut.isSelectedApplicationSaved)
  }

  @Test("isSelectedApplicationSaved is true when selected application is in saved set")
  func isSelectedApplicationSaved_savedApp_returnsTrue() async {
    let (sut, _) = makeSUT(savedApplicationUids: ["APP-001"])
    await sut.loadApplications()
    await sut.activateSavedFilter()
    sut.selectApplication(PlanningApplicationId("APP-001"))

    #expect(sut.isSelectedApplicationSaved)
  }

  @Test("isSelectedApplicationSaved is false when selected application is not in saved set")
  func isSelectedApplicationSaved_unsavedApp_returnsFalse() async {
    let (sut, _) = makeSUT(savedApplicationUids: ["APP-002"])
    await sut.loadApplications()
    await sut.activateSavedFilter()
    sut.selectApplication(PlanningApplicationId("APP-001"))

    #expect(!sut.isSelectedApplicationSaved)
  }
}
