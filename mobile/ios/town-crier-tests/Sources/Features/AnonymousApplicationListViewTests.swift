import Foundation
import Testing
import TownCrierDomain

@testable import TownCrierPresentation

@Suite("AnonymousApplicationListView")
@MainActor
struct AnonymousApplicationListViewTests {
  private func makeViewModel(
    coordinate: Coordinate = .cambridge, radiusMetres: Double = 2000
  ) -> (AnonymousApplicationListViewModel, SpyAnonymousApplicationsRepository) {
    let repository = SpyAnonymousApplicationsRepository()
    let viewModel = AnonymousApplicationListViewModel(
      repository: repository,
      zoneRepository: SpyDeviceLocalZoneRepository(),
      fallbackCoordinate: coordinate,
      fallbackRadiusMetres: radiusMetres)
    return (viewModel, repository)
  }

  @Test func body_renders_whenEmpty() {
    let (viewModel, _) = makeViewModel()
    let sut = AnonymousApplicationListView(viewModel: viewModel)

    _ = sut.body
  }

  @Test func body_renders_withApplicationsLoaded() async {
    let (viewModel, repository) = makeViewModel()
    repository.fetchNearbyResult = .success([.pendingReview, .permitted])
    await viewModel.loadApplications()
    let sut = AnonymousApplicationListView(viewModel: viewModel)

    _ = sut.body
  }

  @Test func body_renders_withErrorState() async {
    let (viewModel, repository) = makeViewModel()
    repository.fetchNearbyResult = .failure(DomainError.networkUnavailable)
    await viewModel.loadApplications()
    let sut = AnonymousApplicationListView(viewModel: viewModel)

    _ = sut.body
  }

  @Test func rowTap_invokesOnShowApplicationDetail() {
    let (viewModel, _) = makeViewModel()
    var captured: [PlanningApplication] = []
    viewModel.onShowApplicationDetail = { captured.append($0) }

    // Mirrors the row's `.onTapGesture` wiring — the tap gesture itself is
    // not exercisable without UI-level automation, so this asserts the same
    // ViewModel call the gesture invokes (mirrors `ApplicationListView`'s
    // reliance on `selectApplication` at the ViewModel level).
    viewModel.selectApplication(.pendingReview)

    #expect(captured == [.pendingReview])
  }

  // MARK: - Zone picker chips (GH#879 Phase 4)

  @Test func body_renders_withZonePickerVisible() async throws {
    let (viewModel, repository) = makeViewModel()
    let zoneRepository = SpyDeviceLocalZoneRepository()
    zoneRepository.loadAllResult = [
      try DeviceLocalZone(name: "Home", centre: .cambridge, radiusMetres: 1000),
      try DeviceLocalZone(name: "Office", centre: .cambridge, radiusMetres: 1000),
    ]
    let vm = AnonymousApplicationListViewModel(
      repository: repository,
      zoneRepository: zoneRepository,
      fallbackCoordinate: .cambridge,
      fallbackRadiusMetres: 2000)
    repository.fetchNearbyResult = .success([])
    await vm.loadApplications()
    #expect(vm.showZonePicker)
    let sut = AnonymousApplicationListView(viewModel: vm)

    _ = sut.body
  }

  @Test func chipTap_invokesSelectZone() async throws {
    let (_, repository) = makeViewModel()
    let zoneRepository = SpyDeviceLocalZoneRepository()
    let zoneA = try DeviceLocalZone(name: "Home", centre: .cambridge, radiusMetres: 1000)
    let zoneB = try DeviceLocalZone(name: "Office", centre: .cambridge, radiusMetres: 1000)
    zoneRepository.loadAllResult = [zoneA, zoneB]
    let vm = AnonymousApplicationListViewModel(
      repository: repository,
      zoneRepository: zoneRepository,
      fallbackCoordinate: .cambridge,
      fallbackRadiusMetres: 2000)
    repository.fetchNearbyResult = .success([])
    await vm.loadApplications()

    // Mirrors the row-tap test above: the chip's tap gesture itself is not
    // exercisable without UI-level automation, so this asserts the same
    // ViewModel call the tap invokes.
    await vm.selectZone(zoneB)

    #expect(vm.selectedZone == zoneB)
  }
}
