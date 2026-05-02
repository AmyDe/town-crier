import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("MapViewModel -- Save from Summary Sheet")
@MainActor
struct MapViewModelSaveTests {
  private static let allApps: [PlanningApplication] = [
    .pendingReview, .permitted, .rejected, .withdrawn,
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
    await sut.loadSavedStateForSelectedApplication()
    sut.selectApplication(PlanningApplicationId("APP-001"))

    #expect(sut.isSelectedApplicationSaved)
  }

  @Test("isSelectedApplicationSaved is false when selected application is not in saved set")
  func isSelectedApplicationSaved_unsavedApp_returnsFalse() async {
    let (sut, _) = makeSUT(savedApplicationUids: ["APP-002"])
    await sut.loadApplications()
    await sut.loadSavedStateForSelectedApplication()
    sut.selectApplication(PlanningApplicationId("APP-001"))

    #expect(!sut.isSelectedApplicationSaved)
  }

  // MARK: - toggleSaveSelectedApplication

  @Test("toggleSaveSelectedApplication saves unsaved application")
  func toggleSave_unsavedApp_callsSave() async {
    let (sut, spy) = makeSUT()
    await sut.loadApplications()
    sut.selectApplication(PlanningApplicationId("APP-001"))

    await sut.toggleSaveSelectedApplication()

    #expect(spy.saveCalls.count == 1)
    #expect(spy.saveCalls[0].id == PlanningApplicationId("APP-001"))
    #expect(sut.isSelectedApplicationSaved)
  }

  @Test("toggleSaveSelectedApplication removes saved application")
  func toggleSave_savedApp_callsRemove() async {
    let (sut, spy) = makeSUT(savedApplicationUids: ["APP-001"])
    await sut.loadApplications()
    await sut.loadSavedStateForSelectedApplication()
    sut.selectApplication(PlanningApplicationId("APP-001"))

    await sut.toggleSaveSelectedApplication()

    #expect(spy.removeCalls == ["APP-001"])
    #expect(!sut.isSelectedApplicationSaved)
  }

  @Test("toggleSaveSelectedApplication is no-op when no application selected")
  func toggleSave_noSelection_isNoOp() async {
    let (sut, spy) = makeSUT()
    await sut.loadApplications()

    await sut.toggleSaveSelectedApplication()

    #expect(spy.saveCalls.isEmpty)
    #expect(spy.removeCalls.isEmpty)
  }

  @Test("toggleSaveSelectedApplication preserves state on save failure")
  func toggleSave_saveFailure_preservesState() async {
    let (sut, spy) = makeSUT()
    spy.saveResult = .failure(DomainError.networkUnavailable)
    await sut.loadApplications()
    sut.selectApplication(PlanningApplicationId("APP-001"))

    await sut.toggleSaveSelectedApplication()

    #expect(!sut.isSelectedApplicationSaved)
  }

  @Test("toggleSaveSelectedApplication preserves state on remove failure")
  func toggleSave_removeFailure_preservesState() async {
    let (sut, spy) = makeSUT(savedApplicationUids: ["APP-001"])
    spy.removeResult = .failure(DomainError.networkUnavailable)
    await sut.loadApplications()
    await sut.loadSavedStateForSelectedApplication()
    sut.selectApplication(PlanningApplicationId("APP-001"))

    await sut.toggleSaveSelectedApplication()

    #expect(sut.isSelectedApplicationSaved)
  }

  // MARK: - loadSavedStateForSelectedApplication

  @Test("loadSavedStateForSelectedApplication populates savedApplicationUids")
  func loadSavedState_populatesUids() async {
    let (sut, spy) = makeSUT()
    spy.loadAllResult = .success([
      SavedApplication(applicationUid: "APP-001", savedAt: Date()),
    ])
    await sut.loadApplications()

    await sut.loadSavedStateForSelectedApplication()

    #expect(sut.savedApplicationUids.contains("APP-001"))
  }

  @Test("loadSavedStateForSelectedApplication is no-op without repository")
  func loadSavedState_noRepository_isNoOp() async {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let sut = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy
    )
    await sut.loadApplications()

    await sut.loadSavedStateForSelectedApplication()

    #expect(sut.savedApplicationUids.isEmpty)
  }

  @Test("loadSavedStateForSelectedApplication leaves empty set on failure")
  func loadSavedState_failure_leavesEmpty() async {
    let (sut, spy) = makeSUT()
    spy.loadAllResult = .failure(DomainError.networkUnavailable)
    await sut.loadApplications()

    await sut.loadSavedStateForSelectedApplication()

    #expect(sut.savedApplicationUids.isEmpty)
  }

  // MARK: - No repository

  @Test("toggleSaveSelectedApplication is no-op without repository")
  func toggleSave_noRepository_isNoOp() async {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(Self.allApps)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let sut = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy
    )
    await sut.loadApplications()
    sut.selectApplication(PlanningApplicationId("APP-001"))

    await sut.toggleSaveSelectedApplication()

    #expect(!sut.isSelectedApplicationSaved)
  }
}
