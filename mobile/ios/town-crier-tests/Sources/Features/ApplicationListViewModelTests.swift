import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("ApplicationListViewModel")
@MainActor
struct ApplicationListViewModelTests {
  private func makeSUT(
    applications: [PlanningApplication] = []
  ) throws -> (ApplicationListViewModel, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchApplicationsResult = .success(applications)
    let vm = ApplicationListViewModel(
      repository: spy,
      zone: .cambridge
    )
    return (vm, spy)
  }

  // MARK: - Loading
  @Test func loadApplications_preservesServerReturnedOrder() async throws {
    // The server owns ordering for every sort now (GH#682 slice 3): the client
    // renders the API order verbatim and never re-sorts locally — that would
    // only ever order the pages already loaded. This input is deliberately not
    // in date order so a leftover client-side `recent-activity` sort would
    // reorder it and fail this assertion.
    let serverOrdered: [PlanningApplication] = [.permitted, .pendingReview, .rejected]
    let (sut, _) = try makeSUT(applications: serverOrdered)

    await sut.loadApplications()

    #expect(sut.filteredApplications.map(\.id) == serverOrdered.map(\.id))
  }

  @Test func loadApplications_setsIsLoadingFalseAfterFetch() async throws {
    let (sut, _) = try makeSUT()

    #expect(!sut.isLoading)
    await sut.loadApplications()
    #expect(!sut.isLoading)
  }

  @Test func loadApplications_setsErrorOnFailure() async throws {
    let (sut, spy) = try makeSUT()
    // The list path is paged now (GH#682 slice 3), so the error must surface
    // from the paged fetch — `fetchApplicationsResult` only feeds the spy's
    // param-less fallback, which the page path no longer uses.
    spy.fetchApplicationsPageError = DomainError.networkUnavailable

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.filteredApplications.isEmpty)
  }

  @Test func loadApplications_clearsErrorOnRetry() async throws {
    let (sut, spy) = try makeSUT()
    spy.fetchApplicationsPageError = DomainError.networkUnavailable
    await sut.loadApplications()

    spy.fetchApplicationsPageError = nil
    spy.fetchApplicationsResult = .success([.pendingReview])
    await sut.loadApplications()

    #expect(sut.error == nil)
    #expect(sut.filteredApplications.count == 1)
  }

  @Test func loadApplications_callsRepositoryWithZone() async throws {
    let (sut, spy) = try makeSUT()

    await sut.loadApplications()

    #expect(spy.fetchApplicationsPageCalls.count == 1)
    #expect(spy.fetchApplicationsPageCalls.first?.zone.id == WatchZone.cambridge.id)
  }

  // MARK: - Selection

  @Test func selectApplication_notifiesCallback() async throws {
    var selectedId: PlanningApplicationId?
    let (sut, _) = try makeSUT(applications: [.pendingReview])
    sut.onApplicationSelected = { selectedId = $0 }

    sut.selectApplication(PlanningApplicationId(authority: "CAM", name: "2026/0042"))

    #expect(selectedId == PlanningApplicationId(authority: "CAM", name: "2026/0042"))
  }

  // Status/unread filtering moved server-side in GH#682 slice 4 — the chips and
  // Unread toggle now drive `?status=`/`?unread=` and the ViewModel renders the
  // server's returned set verbatim. That behaviour lives in
  // `ApplicationListViewModelFilterTests`; there is no longer any client-side
  // `filterApplications` to exercise here.

  // MARK: - Zone Resolution

  @Test func loadApplications_withWatchZoneRepository_resolvesFirstZoneAndFetches() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([.pendingReview])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge, .london])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )

    await sut.loadApplications()

    #expect(zoneSpy.loadAllCallCount == 1)
    #expect(appSpy.fetchApplicationsPageCalls.count == 1)
    #expect(appSpy.fetchApplicationsPageCalls.first?.zone.id == WatchZone.cambridge.id)
    #expect(sut.filteredApplications.count == 1)
  }

  @Test func loadApplications_withWatchZoneRepository_noZones_showsEmptyNotError() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )

    await sut.loadApplications()

    #expect(sut.filteredApplications.isEmpty)
    #expect(sut.error == nil)
    #expect(!sut.isLoading)
  }

  @Test func loadApplications_withWatchZoneRepository_zoneFetchFails_setsError() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .failure(DomainError.networkUnavailable)
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(appSpy.fetchApplicationsPageCalls.isEmpty)
  }

  @Test func loadApplications_withWatchZoneRepository_refreshesZonesOnEveryCall() async throws {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success([.pendingReview])
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success([.cambridge])
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )

    await sut.loadApplications()
    await sut.loadApplications()

    #expect(zoneSpy.loadAllCallCount == 2)
    #expect(appSpy.fetchApplicationsPageCalls.count == 2)
  }

  // MARK: - Empty State
  @Test func isEmpty_trueWhenNoApplicationsLoaded() async throws {
    let (sut, _) = try makeSUT()
    await sut.loadApplications()

    #expect(sut.isEmpty)
  }

  @Test func isEmpty_falseWhenApplicationsExist() async throws {
    let (sut, _) = try makeSUT(applications: [.pendingReview])
    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  // MARK: - Zone Selection

  private func makeSUTWithZones(
    zones: [WatchZone] = [.cambridge, .london],
    applications: [PlanningApplication] = [.pendingReview],
    persistedZoneId: String? = nil
  ) throws -> (
    ApplicationListViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository, UserDefaults
  ) {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchApplicationsResult = .success(applications)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    if let persistedZoneId {
      defaults.set(persistedZoneId, forKey: "test.zone")
    }
    let vm = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: appSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )
    return (vm, appSpy, zoneSpy, defaults)
  }

  @Test func loadApplications_populatesZonesFromRepository() async throws {
    let (sut, _, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    #expect(sut.zones.count == 2)
    #expect(sut.zones[0].id == WatchZone.cambridge.id)
    #expect(sut.zones[1].id == WatchZone.london.id)
  }

  @Test func loadApplications_selectsFirstZoneByDefault() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchApplicationsPageCalls.first?.zone.id == WatchZone.cambridge.id)
  }

  @Test func loadApplications_restoresPersistedZoneSelection() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones(persistedZoneId: "zone-002")
    await sut.loadApplications()
    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchApplicationsPageCalls.first?.zone.id == WatchZone.london.id)
  }

  @Test func loadApplications_fallsBackToFirstZone_whenPersistedZoneDeleted() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones(persistedZoneId: "zone-deleted")
    await sut.loadApplications()
    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchApplicationsPageCalls.first?.zone.id == WatchZone.cambridge.id)
  }

  @Test func selectZone_fetchesApplicationsForNewZone() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    let callsBefore = appSpy.fetchApplicationsPageCalls.count
    await sut.selectZone(.london)
    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchApplicationsPageCalls.count == callsBefore + 1)
    #expect(appSpy.fetchApplicationsPageCalls.last?.zone.id == WatchZone.london.id)
  }

  @Test func selectZone_persistsSelectionToUserDefaults() async throws {
    let (sut, _, _, defaults) = try makeSUTWithZones()
    await sut.loadApplications()
    await sut.selectZone(.london)
    #expect(defaults.string(forKey: "test.zone") == "zone-002")
  }

  @Test func selectZone_resetsStatusFilter() async throws {
    let (sut, _, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    sut.selectedStatusFilter = .permitted
    await sut.selectZone(.london)
    #expect(sut.selectedStatusFilter == nil)
  }

  // MARK: - Zone Picker Visibility (tc-acf0: 'All' chip removed)

  @Test func showZonePicker_trueWhenMultipleZones() async throws {
    let (sut, _, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    #expect(sut.showZonePicker)
  }

  @Test func showZonePicker_falseWithSingleZone() async throws {
    let (sut, _, _, _) = try makeSUTWithZones(zones: [.cambridge])
    await sut.loadApplications()
    #expect(!sut.showZonePicker)
  }

  @Test func showZonePicker_falseWithNoZones() async throws {
    let (sut, _, _, _) = try makeSUTWithZones(zones: [])
    await sut.loadApplications()
    #expect(!sut.showZonePicker)
  }
}
