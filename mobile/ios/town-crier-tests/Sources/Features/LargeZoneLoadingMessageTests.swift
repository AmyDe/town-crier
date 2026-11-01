import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@MainActor
private func settle() async {
  for _ in 0..<20 { await Task.yield() }
}

private func zone(id: String, radiusMetres: Double) throws -> WatchZone {
  try WatchZone(
    id: WatchZoneId(id), name: id, centre: .cambridge, radiusMetres: radiusMetres)
}

@Suite("MapViewModel large zone loading message")
@MainActor
struct MapViewModelLargeZoneLoadingMessageTests {
  private func makeSUT(
    zones: [WatchZone]
  ) -> (MapViewModel, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let sut = MapViewModel(
      repository: spy,
      watchZoneRepository: zoneSpy,
      userDefaults: UserDefaults(suiteName: UUID().uuidString) ?? .standard,
      slowLoadDelay: .zero
    )
    return (sut, spy)
  }

  @Test func showsMessage_forLargeZone_whileLoadIsSlow() async throws {
    let (sut, spy) = makeSUT(zones: [try zone(id: "large", radiusMetres: 5000)])
    spy.enableGate()

    let load = Task { await sut.loadApplications() }
    await settle()

    #expect(sut.showsLargeZoneLoadingMessage == true)
    spy.releaseGate()
    await load.value
  }

  @Test func hidesMessage_forSmallZone_evenWhenLoadIsSlow() async throws {
    let (sut, spy) = makeSUT(zones: [try zone(id: "small", radiusMetres: 500)])
    spy.enableGate()

    let load = Task { await sut.loadApplications() }
    await settle()

    #expect(sut.showsLargeZoneLoadingMessage == false)
    spy.releaseGate()
    await load.value
  }

  @Test func hidesMessage_afterLoadEnds() async throws {
    let (sut, spy) = makeSUT(zones: [try zone(id: "large", radiusMetres: 5000)])
    spy.enableGate()
    let load = Task { await sut.loadApplications() }
    await settle()
    spy.releaseGate()
    await load.value
    await settle()

    #expect(sut.showsLargeZoneLoadingMessage == false)
  }

  @Test func hidesMessage_whenZoneChangeSupersedesTheLoad() async throws {
    let large = try zone(id: "large", radiusMetres: 5000)
    let small = try zone(id: "small", radiusMetres: 500)
    let (sut, spy) = makeSUT(zones: [large, small])
    spy.enableGate()
    let first = Task { await sut.loadApplications() }
    await settle()
    #expect(sut.showsLargeZoneLoadingMessage == true)

    let second = Task { await sut.selectZone(small) }
    await settle()

    #expect(sut.showsLargeZoneLoadingMessage == false)
    spy.releaseGate()
    await first.value
    await second.value
    #expect(sut.showsLargeZoneLoadingMessage == false)
  }

  @Test func notice_showsOnlyForLargeSelectedZone() async throws {
    let large = try zone(id: "large", radiusMetres: 5000)
    let small = try zone(id: "small", radiusMetres: 500)
    let (sut, _) = makeSUT(zones: [large, small])
    #expect(sut.showsLargeZoneNotice == false)

    await sut.loadApplications()
    await sut.selectZone(large)
    #expect(sut.showsLargeZoneNotice == true)

    await sut.selectZone(small)
    #expect(sut.showsLargeZoneNotice == false)
  }

  @Test func supersededLoadCompletion_doesNotClearNewerLoad() async throws {
    let large = try zone(id: "large", radiusMetres: 5000)
    let large2 = try zone(id: "large2", radiusMetres: 6000)
    let (sut, spy) = makeSUT(zones: [large, large2])
    spy.enableGate()
    let first = Task { await sut.loadApplications() }
    await settle()
    let second = Task { await sut.selectZone(large2) }
    await settle()
    spy.releaseGate()
    await first.value
    await second.value

    #expect(sut.isLoading == false)
  }
}

@Suite("ApplicationListViewModel large zone loading message")
@MainActor
struct ApplicationListViewModelLargeZoneLoadingMessageTests {
  private func makeSUT(
    zones: [WatchZone]
  ) -> (ApplicationListViewModel, SpyPlanningApplicationRepository) {
    let spy = SpyPlanningApplicationRepository()
    let zoneSpy = SpyWatchZoneRepository()
    zoneSpy.loadAllResult = .success(zones)
    let sut = ApplicationListViewModel(
      watchZoneRepository: zoneSpy,
      repository: spy,
      userDefaults: UserDefaults(suiteName: UUID().uuidString) ?? .standard,
      slowLoadDelay: .zero
    )
    return (sut, spy)
  }

  @Test func showsMessage_forLargeZone_whileLoadIsSlow() async throws {
    let (sut, spy) = makeSUT(zones: [try zone(id: "large", radiusMetres: 5000)])
    spy.enableGate()

    let load = Task { await sut.loadApplications() }
    await settle()

    #expect(sut.showsLargeZoneLoadingMessage == true)
    spy.releaseGate()
    await load.value
  }

  @Test func hidesMessage_forSmallZone_evenWhenLoadIsSlow() async throws {
    let (sut, spy) = makeSUT(zones: [try zone(id: "small", radiusMetres: 500)])
    spy.enableGate()

    let load = Task { await sut.loadApplications() }
    await settle()

    #expect(sut.showsLargeZoneLoadingMessage == false)
    spy.releaseGate()
    await load.value
  }

  @Test func hidesMessage_afterLoadEnds() async throws {
    let (sut, spy) = makeSUT(zones: [try zone(id: "large", radiusMetres: 5000)])
    spy.enableGate()
    let load = Task { await sut.loadApplications() }
    await settle()
    spy.releaseGate()
    await load.value
    await settle()

    #expect(sut.showsLargeZoneLoadingMessage == false)
  }

  @Test func hidesMessage_whenZoneChangeSupersedesTheLoad() async throws {
    let large = try zone(id: "large", radiusMetres: 5000)
    let small = try zone(id: "small", radiusMetres: 500)
    let (sut, spy) = makeSUT(zones: [large, small])
    spy.enableGate()
    let first = Task { await sut.loadApplications() }
    await settle()
    #expect(sut.showsLargeZoneLoadingMessage == true)

    let second = Task { await sut.selectZone(small) }
    await settle()

    #expect(sut.showsLargeZoneLoadingMessage == false)
    spy.releaseGate()
    await first.value
    await second.value
    #expect(sut.showsLargeZoneLoadingMessage == false)
  }

  @Test func notice_showsOnlyForLargeSelectedZone() async throws {
    let large = try zone(id: "large", radiusMetres: 5000)
    let small = try zone(id: "small", radiusMetres: 500)
    let (sut, _) = makeSUT(zones: [large, small])
    #expect(sut.showsLargeZoneNotice == false)

    await sut.loadApplications()
    await sut.selectZone(large)
    #expect(sut.showsLargeZoneNotice == true)

    await sut.selectZone(small)
    #expect(sut.showsLargeZoneNotice == false)
  }

  @Test func supersededLoadCompletion_doesNotClearNewerLoad() async throws {
    let large = try zone(id: "large", radiusMetres: 5000)
    let large2 = try zone(id: "large2", radiusMetres: 6000)
    let (sut, spy) = makeSUT(zones: [large, large2])
    spy.enableGate()
    let first = Task { await sut.loadApplications() }
    await settle()
    let second = Task { await sut.selectZone(large2) }
    await settle()
    spy.releaseGate()
    await first.value
    await second.value

    #expect(sut.isLoading == false)
  }
}
