import Foundation
import Testing
@testable import TownCrierDomain
@testable import TownCrierPresentation

@MainActor
@Suite("WatchZoneListViewModel")
struct WatchZoneListViewModelTests {
    private var spyRepository: SpyWatchZoneRepository!
    private var spySubscriptionService: SpySubscriptionService!
    private var sut: WatchZoneListViewModel!

    init() {
        spyRepository = SpyWatchZoneRepository()
        spySubscriptionService = SpySubscriptionService()
        sut = WatchZoneListViewModel(
            repository: spyRepository,
            subscriptionService: spySubscriptionService
        )
    }

    // MARK: - Loading

    @Test func load_populatesZones() async {
        spyRepository.loadAllResult = .success([.cambridge])

        await sut.load()

        #expect(sut.zones == [.cambridge])
        #expect(!sut.isLoading)
        #expect(sut.error == nil)
    }

    @Test func load_setsErrorOnFailure() async {
        spyRepository.loadAllResult = .failure(DomainError.networkUnavailable)

        await sut.load()

        #expect(sut.zones.isEmpty)
        #expect(sut.error == .networkUnavailable)
    }

    @Test func load_resolvesCurrentTier() async {
        spySubscriptionService.currentEntitlementResult = .personalActive

        await sut.load()

        #expect(sut.currentTier == .personal)
    }

    @Test func load_noEntitlement_defaultsToFree() async {
        spySubscriptionService.currentEntitlementResult = nil

        await sut.load()

        #expect(sut.currentTier == .free)
    }

    // MARK: - Tier limits

    @Test func canAddZone_freeWithNoZones_returnsTrue() async {
        spySubscriptionService.currentEntitlementResult = nil
        spyRepository.loadAllResult = .success([])

        await sut.load()

        #expect(sut.canAddZone)
    }

    @Test func canAddZone_freeWithOneZone_returnsFalse() async {
        spySubscriptionService.currentEntitlementResult = nil
        spyRepository.loadAllResult = .success([.cambridge])

        await sut.load()

        #expect(!sut.canAddZone)
    }

    @Test func canAddZone_proWithManyZones_returnsTrue() async {
        spySubscriptionService.currentEntitlementResult = .proActive
        spyRepository.loadAllResult = .success([.cambridge, .london])

        await sut.load()

        #expect(sut.canAddZone)
    }

    // MARK: - Delete

    @Test func deleteZone_callsRepositoryAndRemovesFromList() async {
        spyRepository.loadAllResult = .success([.cambridge, .london])
        await sut.load()

        await sut.deleteZone(.cambridge)

        #expect(spyRepository.deleteCalls == [WatchZoneId("zone-001")])
        #expect(sut.zones == [.london])
    }

    @Test func deleteZone_onFailure_setsError() async {
        spyRepository.loadAllResult = .success([.cambridge])
        await sut.load()
        spyRepository.deleteResult = .failure(DomainError.networkUnavailable)

        await sut.deleteZone(.cambridge)

        #expect(sut.error == .networkUnavailable)
        #expect(sut.zones == [.cambridge])
    }

    // MARK: - Navigation callbacks

    @Test func addZone_invokesOnAddZone() async {
        var addCalled = false
        sut.onAddZone = { addCalled = true }

        sut.addZone()

        #expect(addCalled)
    }

    @Test func editZone_invokesOnEditZone() async {
        var editedZone: WatchZone?
        sut.onEditZone = { editedZone = $0 }

        sut.editZone(.cambridge)

        #expect(editedZone == .cambridge)
    }

    @Test func addZone_atLimit_invokesOnUpgradeRequired() async {
        spySubscriptionService.currentEntitlementResult = nil
        spyRepository.loadAllResult = .success([.cambridge])
        await sut.load()
        var upgradeCalled = false
        sut.onUpgradeRequired = { upgradeCalled = true }

        sut.addZone()

        #expect(upgradeCalled)
    }
}
