import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

/// GH#879 Phase 3: the anonymous Applications tab's list ViewModel — a
/// single nearest-first page over `AnonymousApplicationsRepository`, no
/// sort/filter chips (pre-resolved: v1 is nearest-first only).
///
/// GH#879 Phase 4: the active `DeviceLocalZone` now drives the query, with a
/// zone picker mirroring the authed `ApplicationListViewModel`'s pattern.
/// `fallbackCoordinate`/`fallbackRadiusMetres` back the query only in the
/// practically-unreachable case no device-local zone exists at all.
@Suite("AnonymousApplicationListViewModel")
@MainActor
struct AnonymousApplicationListViewModelTests {
  private func makeSUT(
    fallbackCoordinate: Coordinate = .cambridge,
    fallbackRadiusMetres: Double = 2000,
    zones: [DeviceLocalZone] = [],
    activeZoneId: DeviceLocalZoneId? = nil
  ) -> (AnonymousApplicationListViewModel, SpyAnonymousApplicationsRepository, SpyDeviceLocalZoneRepository) {
    let repository = SpyAnonymousApplicationsRepository()
    let zoneRepository = SpyDeviceLocalZoneRepository()
    zoneRepository.loadAllResult = zones
    zoneRepository.activeZoneIdResult = activeZoneId
    let sut = AnonymousApplicationListViewModel(
      repository: repository,
      zoneRepository: zoneRepository,
      fallbackCoordinate: fallbackCoordinate,
      fallbackRadiusMetres: fallbackRadiusMetres
    )
    return (sut, repository, zoneRepository)
  }

  private func makeZone(
    name: String = "Home", radiusMetres: Double = 1500
  ) throws -> DeviceLocalZone {
    try DeviceLocalZone(name: name, centre: .cambridge, radiusMetres: radiusMetres)
  }

  // MARK: - loadApplications, no device-local zones (fallback)

  @Test func loadApplications_noZones_fetchesAtFallbackCoordinateAndRadius() async {
    let (sut, repository, _) = makeSUT(
      fallbackCoordinate: .cambridge, fallbackRadiusMetres: 1500, zones: [])
    repository.fetchNearbyResult = .success([.pendingReview, .permitted])

    await sut.loadApplications()

    #expect(repository.fetchNearbyCalls.count == 1)
    let call = repository.fetchNearbyCalls[0]
    #expect(call.latitude == Coordinate.cambridge.latitude)
    #expect(call.longitude == Coordinate.cambridge.longitude)
    #expect(call.radiusMetres == 1500)
    #expect(call.limit == AnonymousApplicationListViewModel.defaultLimit)
    #expect(sut.applications == [.pendingReview, .permitted])
  }

  @Test func loadApplications_setsIsLoadingFalseAfterCompletion() async {
    let (sut, _, _) = makeSUT()

    #expect(!sut.isLoading)
    await sut.loadApplications()
    #expect(!sut.isLoading)
  }

  @Test func loadApplications_failure_setsError() async {
    let (sut, repository, _) = makeSUT()
    repository.fetchNearbyResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(sut.applications.isEmpty)
    #expect(sut.error == .networkUnavailable)
  }

  /// Pull-to-refresh calls the same `loadApplications()` entry point — a
  /// second successful fetch replaces the previously loaded rows.
  @Test func loadApplications_calledAgain_replacesPreviousApplications() async {
    let (sut, repository, _) = makeSUT()
    repository.fetchNearbyResult = .success([.pendingReview])
    await sut.loadApplications()
    #expect(sut.applications == [.pendingReview])

    repository.fetchNearbyResult = .success([.permitted])
    await sut.loadApplications()

    #expect(sut.applications == [.permitted])
    #expect(repository.fetchNearbyCalls.count == 2)
  }

  // MARK: - isEmpty

  @Test func isEmpty_trueWhenNoApplicationsNoErrorNotLoading() async {
    let (sut, repository, _) = makeSUT()
    repository.fetchNearbyResult = .success([])

    await sut.loadApplications()

    #expect(sut.isEmpty)
  }

  @Test func isEmpty_falseWhenApplicationsPresent() async {
    let (sut, repository, _) = makeSUT()
    repository.fetchNearbyResult = .success([.pendingReview])

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  @Test func isEmpty_falseWhenErrorPresent() async {
    let (sut, repository, _) = makeSUT()
    repository.fetchNearbyResult = .failure(DomainError.networkUnavailable)

    await sut.loadApplications()

    #expect(!sut.isEmpty)
  }

  // MARK: - Row selection -> detail handoff (GH#879 Phase 2 established handoff)

  @Test func selectApplication_invokesOnShowApplicationDetail() {
    let (sut, _, _) = makeSUT()
    var captured: [PlanningApplication] = []
    sut.onShowApplicationDetail = { captured.append($0) }

    sut.selectApplication(.pendingReview)

    #expect(captured == [.pendingReview])
  }

  // MARK: - Zone-driven browsing (GH#879 Phase 4)

  @Test func loadApplications_withActiveZone_fetchesAtZoneCoordinateAndRadius() async throws {
    let zone = try makeZone(radiusMetres: 3000)
    let (sut, repository, _) = makeSUT(
      fallbackCoordinate: .cambridge,
      fallbackRadiusMetres: 2000,
      zones: [zone],
      activeZoneId: zone.id)
    repository.fetchNearbyResult = .success([])

    await sut.loadApplications()

    #expect(repository.fetchNearbyCalls.last?.radiusMetres == 3000)
    #expect(sut.selectedZone == zone)
  }

  @Test func loadApplications_noActiveZoneIdSet_defaultsToFirstZone() async throws {
    let zone = try makeZone()
    let (sut, repository, _) = makeSUT(zones: [zone], activeZoneId: nil)
    repository.fetchNearbyResult = .success([])

    await sut.loadApplications()

    #expect(sut.selectedZone == zone)
  }

  @Test func showZonePicker_false_whenZeroOrOneZone() async throws {
    let (sutZero, repoZero, _) = makeSUT(zones: [])
    repoZero.fetchNearbyResult = .success([])
    await sutZero.loadApplications()
    #expect(!sutZero.showZonePicker)

    let zone = try makeZone()
    let (sutOne, repoOne, _) = makeSUT(zones: [zone])
    repoOne.fetchNearbyResult = .success([])
    await sutOne.loadApplications()
    #expect(!sutOne.showZonePicker)
  }

  @Test func showZonePicker_true_whenMultipleZones() async throws {
    let (sut, repository, _) = makeSUT(
      zones: [try makeZone(name: "One"), try makeZone(name: "Two")])
    repository.fetchNearbyResult = .success([])

    await sut.loadApplications()

    #expect(sut.showZonePicker)
  }

  @Test func selectZone_persistsActiveZoneAndRefetches() async throws {
    let zoneA = try makeZone(name: "A", radiusMetres: 1000)
    let zoneB = try makeZone(name: "B", radiusMetres: 4000)
    let (sut, repository, zoneRepository) = makeSUT(
      zones: [zoneA, zoneB], activeZoneId: zoneA.id)
    repository.fetchNearbyResult = .success([])
    await sut.loadApplications()

    repository.fetchNearbyResult = .success([.pendingReview])
    await sut.selectZone(zoneB)

    #expect(sut.selectedZone == zoneB)
    #expect(zoneRepository.setActiveZoneIdCalls.last == zoneB.id)
    #expect(repository.fetchNearbyCalls.last?.radiusMetres == 4000)
    #expect(sut.applications == [.pendingReview])
  }

  @Test func selectZone_invokesOnActiveZoneChanged() async throws {
    let zoneA = try makeZone(name: "A")
    let zoneB = try makeZone(name: "B")
    let (sut, repository, _) = makeSUT(zones: [zoneA, zoneB], activeZoneId: zoneA.id)
    repository.fetchNearbyResult = .success([])
    var changed: [DeviceLocalZone] = []
    sut.onActiveZoneChanged = { changed.append($0) }

    await sut.selectZone(zoneB)

    #expect(changed == [zoneB])
  }
}
