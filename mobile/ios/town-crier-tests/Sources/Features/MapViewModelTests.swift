import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("MapViewModel")
@MainActor
struct MapViewModelTests {
  private func makeSUT(
    clusters: [MapCluster] = [],
    watchZones: [WatchZone] = [.cambridge]
  ) -> (MapViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository) {
    let spy = SpyPlanningApplicationRepository()
    spy.fetchClustersResult = .success(clusters)
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success(watchZones)
    let vm = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    return (vm, spy, watchZoneSpy)
  }

  // MARK: - Loading clusters

  @Test func loadApplications_loadsClustersForSelectedZone() async {
    let (sut, _, _) = makeSUT(clusters: [.bubble(count: 50), .single(member: .init(authority: "1", name: "A"))])

    await sut.loadApplications()

    #expect(sut.clusters.count == 2)
  }

  /// The headline of GH#698: the map path no longer eager-drains every page —
  /// it fetches clusters for the viewport instead.
  @Test func loadApplications_doesNotDrainPages() async {
    let (sut, spy, _) = makeSUT(clusters: [.bubble(count: 10)])

    await sut.loadApplications()

    #expect(spy.fetchApplicationsPageCalls.isEmpty)
    #expect(spy.fetchClustersCalls.count == 1)
  }

  @Test func loadApplications_setsIsLoadingDuringFetch() async {
    let (sut, _, _) = makeSUT()

    #expect(!sut.isLoading)
    await sut.loadApplications()
    #expect(!sut.isLoading)
  }

  @Test func loadApplications_setsErrorOnFailure() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchClustersResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.clusters.isEmpty)
  }

  // MARK: - Viewport refetch

  @Test func loadClusters_refetchesForTheGivenViewportAndZoom() async {
    let (sut, spy, _) = makeSUT(clusters: [.bubble(count: 5)])
    await sut.loadApplications()

    await sut.loadClusters(viewport: .test2, zoom: 17)

    let last = spy.fetchClustersCalls.last
    #expect(last?.viewport == .test2)
    #expect(last?.zoom == 17)
  }

  @Test func loadClusters_keepsStaleClustersOnRefetchFailure() async {
    let (sut, spy, _) = makeSUT(clusters: [.bubble(count: 5)])
    await sut.loadApplications()
    #expect(sut.clusters.count == 1)

    spy.fetchClustersResult = .failure(DomainError.networkUnavailable)
    await sut.loadClusters(viewport: .test2, zoom: 17)

    // A transient pan/zoom failure leaves the last good clusters in place and
    // does not nuke the map with a screen-level error.
    #expect(sut.clusters.count == 1)
    #expect(sut.error == nil)
  }

  // MARK: - Watch zone

  @Test func loadApplications_setsCentreFromWatchZone() async throws {
    let zone = try WatchZone(
      postcode: Postcode("SW1A 1AA"),
      centre: Coordinate(latitude: 51.5, longitude: -0.1),
      radiusMetres: 3000
    )
    let (sut, _, _) = makeSUT(watchZones: [zone])

    await sut.loadApplications()

    #expect(sut.centreLat == 51.5)
    #expect(sut.centreLon == -0.1)
    #expect(sut.radiusMetres == 3000)
  }

  @Test func loadApplications_setsError_whenWatchZoneFetchFails() async {
    let (sut, _, watchZoneSpy) = makeSUT()
    watchZoneSpy.loadAllResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.error == .networkUnavailable)
    #expect(sut.centreLat == 51.5074)
    #expect(sut.centreLon == -0.1278)
    #expect(sut.radiusMetres == 2000)
  }

  @Test func loadApplications_fetchesWatchZone() async {
    let (sut, _, watchZoneSpy) = makeSUT()

    await sut.loadApplications()

    #expect(watchZoneSpy.loadAllCallCount == 1)
  }

  // MARK: - Single-member tap selection

  @Test func selectCluster_singleMember_pointReadsAndSelectsTheApplication() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchApplicationResult = .success(.pendingReview)
    await sut.loadApplications()

    await sut.selectCluster(.single(member: PlanningApplication.pendingReview.id))

    #expect(sut.selectedApplication?.id == PlanningApplication.pendingReview.id)
    // Exactly one point read, keyed by the cluster's member id — no held set,
    // no O(n) scan, no full-zone re-drain.
    #expect(spy.fetchApplicationCalls == [PlanningApplication.pendingReview.id])
  }

  @Test func selectCluster_multiMember_doesNotFetchOrSelect() async {
    let (sut, spy, _) = makeSUT()
    await sut.loadApplications()

    await sut.selectCluster(.bubble(count: 42))

    #expect(sut.selectedApplication == nil)
    #expect(spy.fetchApplicationCalls.isEmpty)
  }

  @Test func selectCluster_singleMember_leavesSelectionNil_whenPointReadFails() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchApplicationResult = .failure(DomainError.networkUnavailable)
    await sut.loadApplications()

    await sut.selectCluster(.single(member: PlanningApplication.pendingReview.id))

    #expect(sut.selectedApplication == nil)
  }

  @Test func clearSelection_clearsSelectedApplication() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchApplicationResult = .success(.pendingReview)
    await sut.loadApplications()
    await sut.selectCluster(.single(member: PlanningApplication.pendingReview.id))

    sut.clearSelection()

    #expect(sut.selectedApplication == nil)
  }

  // MARK: - Stacked-cluster selection (GH#722)

  @Test func selectStack_fetchesEachMemberAndPublishesOrderedList() async {
    let (sut, spy, _) = makeSUT(clusters: [.bubble(count: 3)])
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
    let (sut, spy, _) = makeSUT(clusters: [.bubble(count: 3)])
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
    let (sut, spy, _) = makeSUT()
    await sut.loadApplications()

    await sut.selectStack(.bubble(count: 42))

    #expect(sut.stackedApplications == nil)
    #expect(spy.fetchApplicationCalls.isEmpty)
  }

  @Test func selectFromStack_routesThroughPendingToSummary() async {
    let (sut, spy, _) = makeSUT(clusters: [.bubble(count: 2)])
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
    let (sut, spy, _) = makeSUT(clusters: [.bubble(count: 2)])
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

  // MARK: - Empty State

  @Test func isEmpty_trueWhenNoClustersAfterLoad() async {
    let (sut, _, _) = makeSUT(clusters: [])

    await sut.loadApplications()

    #expect(sut.isEmpty)
  }

  @Test func isEmpty_falseWhenClustersExist() async {
    let (sut, _, _) = makeSUT(clusters: [.bubble(count: 3)])

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  @Test func isEmpty_falseWhileLoading() async {
    let (sut, _, _) = makeSUT()

    #expect(!sut.isEmpty)
  }

  @Test func isEmpty_falseWhenErrorOccurred() async {
    let (sut, spy, _) = makeSUT()
    spy.fetchClustersResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  // MARK: - Zone-based loading

  @Test func loadApplications_fetchesClustersBySelectedZone() async throws {
    let zone = try WatchZone(
      id: WatchZoneId("zone-1"),
      name: "Camden",
      centre: Coordinate(latitude: 51.539, longitude: -0.1426),
      radiusMetres: 1000,
      authorityId: 42
    )
    let spy = SpyPlanningApplicationRepository()
    spy.fetchClustersResult = .success([.bubble(count: 9)])
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([zone])

    let sut = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    await sut.loadApplications()

    #expect(spy.fetchClustersCalls.count == 1)
    #expect(spy.fetchClustersCalls[0].zone.id == zone.id)
    #expect(sut.clusters.count == 1)
  }

  @Test func loadApplications_returnsEmpty_whenNoZones() async {
    let spy = SpyPlanningApplicationRepository()
    let watchZoneSpy = SpyWatchZoneRepository()
    watchZoneSpy.loadAllResult = .success([])

    let sut = MapViewModel(repository: spy, watchZoneRepository: watchZoneSpy)
    await sut.loadApplications()

    #expect(spy.fetchClustersCalls.isEmpty)
    #expect(sut.clusters.isEmpty)
    #expect(sut.isEmpty)
  }

  // MARK: - Zone Selection

  private func makeSUTWithZones(
    zones: [WatchZone] = [.cambridge, .london],
    clusters: [MapCluster] = [.bubble(count: 3)],
    persistedZoneId: String? = nil
  ) throws -> (MapViewModel, SpyPlanningApplicationRepository, SpyWatchZoneRepository, UserDefaults) {
    let appSpy = SpyPlanningApplicationRepository()
    appSpy.fetchClustersResult = .success(clusters)
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let defaults = try #require(UserDefaults(suiteName: UUID().uuidString))
    if let persistedZoneId {
      defaults.set(persistedZoneId, forKey: "test.zone")
    }
    let vm = MapViewModel(
      repository: appSpy,
      watchZoneRepository: zoneSpy,
      userDefaults: defaults,
      zoneSelectionKey: "test.zone"
    )
    return (vm, appSpy, zoneSpy, defaults)
  }

  @Test func loadApplications_populatesZones() async throws {
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
    #expect(appSpy.fetchClustersCalls.first?.zone.id == WatchZone.cambridge.id)
  }

  @Test func loadApplications_restoresPersistedZoneSelection() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones(persistedZoneId: "zone-002")

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchClustersCalls.first?.zone.id == WatchZone.london.id)
    #expect(sut.centreLat == WatchZone.london.centre.latitude)
    #expect(sut.centreLon == WatchZone.london.centre.longitude)
  }

  @Test func loadApplications_fallsBackToFirstZone_whenPersistedZoneDeleted() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones(persistedZoneId: "zone-deleted")

    await sut.loadApplications()

    #expect(sut.selectedZone?.id == WatchZone.cambridge.id)
    #expect(appSpy.fetchClustersCalls.first?.zone.id == WatchZone.cambridge.id)
  }

  @Test func selectZone_fetchesClustersAndUpdatesCentre() async throws {
    let (sut, appSpy, _, _) = try makeSUTWithZones()
    await sut.loadApplications()
    let initialCallCount = appSpy.fetchClustersCalls.count

    await sut.selectZone(.london)

    #expect(sut.selectedZone?.id == WatchZone.london.id)
    #expect(appSpy.fetchClustersCalls.count == initialCallCount + 1)
    #expect(appSpy.fetchClustersCalls.last?.zone.id == WatchZone.london.id)
    #expect(sut.centreLat == WatchZone.london.centre.latitude)
    #expect(sut.centreLon == WatchZone.london.centre.longitude)
    #expect(sut.radiusMetres == WatchZone.london.radiusMetres)
  }

  @Test func selectZone_persistsSelectionToUserDefaults() async throws {
    let (sut, _, _, defaults) = try makeSUTWithZones()
    await sut.loadApplications()

    await sut.selectZone(.london)

    #expect(defaults.string(forKey: "test.zone") == "zone-002")
  }

  @Test func showZonePicker_trueWhenMultipleZones() async throws {
    let (sut, _, _, _) = try makeSUTWithZones()
    await sut.loadApplications()

    #expect(sut.showZonePicker)
  }

  @Test func showZonePicker_falseWhenSingleZone() async throws {
    let (sut, _, _, _) = try makeSUTWithZones(zones: [.cambridge])
    await sut.loadApplications()

    #expect(!sut.showZonePicker)
  }
}
