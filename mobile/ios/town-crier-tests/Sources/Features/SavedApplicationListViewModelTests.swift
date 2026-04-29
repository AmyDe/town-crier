import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// Tests for the dedicated Saved tab view model — a flat, cross-zone list of
/// the user's bookmarked applications, sorted most-recently-saved first, with
/// a status filter that is free for all subscription tiers.
@Suite("SavedApplicationListViewModel")
@MainActor
struct SavedApplicationListViewModelTests {
  // MARK: - Loading

  @Test func loadAll_populatesAndSortsBySavedAtDesc() async throws {
    let older = SavedApplication.fixture(
      uid: "APP-A",
      savedAt: Date(timeIntervalSince1970: 1000)
    )
    let newer = SavedApplication.fixture(
      uid: "APP-B",
      savedAt: Date(timeIntervalSince1970: 2000)
    )
    let repo = SpySavedApplicationRepository()
    repo.loadAllResult = .success([older, newer])
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)

    await sut.loadAll()

    #expect(sut.applications.map(\.id.value) == ["APP-B", "APP-A"])
    #expect(!sut.isEmpty)
    #expect(sut.error == nil)
    #expect(!sut.isLoading)
  }

  @Test func loadAll_setsIsEmpty_whenRepositoryReturnsNothing() async throws {
    let repo = SpySavedApplicationRepository()
    repo.loadAllResult = .success([])
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)

    await sut.loadAll()

    #expect(sut.applications.isEmpty)
    #expect(sut.isEmpty)
    #expect(sut.error == nil)
  }

  @Test func loadAll_dropsSavedEntriesWithoutDenormalisedPayload() async throws {
    // Legacy saves predate the denormalised payload — they have a uid but no
    // PlanningApplication. They should not appear in the cross-zone list.
    let withPayload = SavedApplication.fixture(uid: "APP-A")
    let withoutPayload = SavedApplication(
      applicationUid: "APP-LEGACY",
      savedAt: Date(timeIntervalSince1970: 5000),
      application: nil
    )
    let repo = SpySavedApplicationRepository()
    repo.loadAllResult = .success([withPayload, withoutPayload])
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)

    await sut.loadAll()

    #expect(sut.applications.map(\.id.value) == ["APP-A"])
  }

  @Test func loadAll_capturesRepositoryError() async throws {
    let repo = SpySavedApplicationRepository()
    repo.loadAllResult = .failure(DomainError.networkUnavailable)
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)

    await sut.loadAll()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.applications.isEmpty)
    #expect(!sut.isLoading)
  }

  @Test func loadAll_clearsErrorOnSuccessfulRetry() async throws {
    let repo = SpySavedApplicationRepository()
    repo.loadAllResult = .failure(DomainError.networkUnavailable)
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)
    await sut.loadAll()
    #expect(sut.error == .networkUnavailable)

    repo.loadAllResult = .success([.fixture(uid: "APP-A")])
    await sut.loadAll()

    #expect(sut.error == nil)
    #expect(sut.applications.count == 1)
  }

  // MARK: - Status Filter

  @Test func statusFilter_filtersByStatus_freeForAllTiers() async throws {
    let permitted = SavedApplication.fixture(uid: "APP-A", status: .permitted)
    let pending = SavedApplication.fixture(uid: "APP-B", status: .undecided)
    let repo = SpySavedApplicationRepository()
    repo.loadAllResult = .success([permitted, pending])
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)
    await sut.loadAll()

    sut.selectedStatusFilter = .undecided

    #expect(sut.filteredApplications.map(\.id.value) == ["APP-B"])
  }

  @Test func statusFilter_nilShowsAllApplications() async throws {
    let repo = SpySavedApplicationRepository()
    repo.loadAllResult = .success([
      .fixture(uid: "APP-A", status: .permitted),
      .fixture(uid: "APP-B", status: .undecided),
    ])
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)
    await sut.loadAll()

    sut.selectedStatusFilter = .permitted
    #expect(sut.filteredApplications.count == 1)

    sut.selectedStatusFilter = nil
    #expect(sut.filteredApplications.count == 2)
  }

  @Test func statusFilter_noMatches_showsEmpty() async throws {
    let repo = SpySavedApplicationRepository()
    repo.loadAllResult = .success([.fixture(uid: "APP-A", status: .permitted)])
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)
    await sut.loadAll()

    sut.selectedStatusFilter = .rejected

    #expect(sut.filteredApplications.isEmpty)
    #expect(sut.isEmpty)
  }

  // MARK: - Selection

  @Test func selectApplication_notifiesCallback() async throws {
    var selectedId: PlanningApplicationId?
    let repo = SpySavedApplicationRepository()
    let sut = SavedApplicationListViewModel(savedApplicationRepository: repo)
    sut.onApplicationSelected = { selectedId = $0 }

    sut.selectApplication(PlanningApplicationId("APP-001"))

    #expect(selectedId == PlanningApplicationId("APP-001"))
  }
}
